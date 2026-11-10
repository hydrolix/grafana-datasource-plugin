package plugin

import (
	"net/http"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	hdxbuild "github.com/hydrolix/plugin/pkg/build"
)

// The cluster is reached through clickhouse-go's own http.Client, one per
// pooled connection, built around the http.Transport it hands to
// Options.TransportFunc. That transport keeps clickhouse-go's dial and read
// timeouts, pooling, proxy and TLS; the plugin does not rebuild it with
// backend/httpclient, whose defaults differ and would have to be reconciled on
// every upgrade of either side. What the SDK's client would add on top of a
// transport is a chain of middlewares, and those take any RoundTripper, so the
// chain is installed here, around clickhouse-go's transport, and Grafana's
// protections for an outgoing HTTP request hold on the query path as they do
// on the token-exchange client (pkg/plugin/exchange/client.go).
//
// Grafana's plugin review of 0.13.1 asked for this.

// sdkMiddlewares is the SDK's default chain (httpclient.DefaultMiddlewares)
// less the two that write credentials: BasicAuthenticationMiddleware and
// CustomHeadersMiddleware set Authorization and configured headers from the
// datasource's own HTTP settings, which this plugin does not use, and either
// would overwrite the Bearer the fork puts on the request in the forwarding
// modes. The list is kept by hand so that a middleware the SDK adds later
// does not join the chain unread.
func sdkMiddlewares() []httpclient.Middleware {
	return []httpclient.Middleware{
		httpclient.TracingMiddleware(nil),
		httpclient.DataSourceMetricsMiddleware(),
		// A no-op today: the fork sets User-Agent from ClientInfo, and the
		// middleware fills the header only when it is absent.
		httpclient.UserAgentMiddleware(),
		httpclient.ContextualMiddleware(),
		httpclient.ErrorSourceMiddleware(),
		// The limit is resolved per request from the request's context:
		// Grafana's dataproxy.response_limit, which the SDK puts on every
		// handler's context (QueryData, CheckHealth, CallResource), and the
		// fork builds each request with that context. The argument is the
		// fallback when the context carries none, and 0 means no limit, which
		// is Grafana's own default; the plugin invents none.
		httpclient.ResponseLimitMiddleware(0),
	}
}

// sdkOptions is what the chain reads from the datasource: the labels the
// metrics middleware keys its series by and the limit middleware logs. The
// metrics middleware is silently a no-op without datasource_type, so the
// plugin ID stands in when the instance settings carry no type.
func sdkOptions(config backend.DataSourceInstanceSettings) httpclient.Options {
	dsType := config.Type
	if dsType == "" {
		dsType = hdxbuild.BuildInfo{}.GetBuildInfo().PluginID
	}
	return httpclient.Options{Labels: map[string]string{
		"datasource_name": config.Name,
		"datasource_uid":  config.UID,
		"datasource_type": dsType,
	}}
}

// withSDKMiddlewares wraps base in the chain, first middleware outermost, as
// httpclient.New does.
func withSDKMiddlewares(opts httpclient.Options, base http.RoundTripper) http.RoundTripper {
	middlewares := sdkMiddlewares()
	next := base
	for i := len(middlewares) - 1; i >= 0; i-- {
		next = middlewares[i].CreateMiddleware(opts, next)
	}
	return next
}

// idleConnCloser puts CloseIdleConnections back on top of the chain. The
// SDK's middlewares are RoundTripperFuncs, which have no such method, and
// http.Client finds it by a type assertion on its transport; without this
// the fork's httpConnect.close() would release nothing, and every retired
// connection would keep its keep-alive socket until the transport's idle
// timeout. The call goes to the transport itself, which owns the sockets.
type idleConnCloser struct {
	http.RoundTripper
	transport *http.Transport
}

var _ interface{ CloseIdleConnections() } = (*idleConnCloser)(nil)

func (c *idleConnCloser) CloseIdleConnections() {
	c.transport.CloseIdleConnections()
}

// transportFor is the one TransportFunc every credentials mode installs.
// clickhouse-go builds its Transport with DisableCompression: true; the plugin
// keeps Go's transparent gzip on, as it always has. Bottom up: no redirect is
// followed, in any mode, and the guard sits directly on the transport so a
// hop is refused on the cluster's raw answer, before any middleware wraps
// the body; the SDK chain is above it; wrap, when given, sits above that, so
// a watch sees the cluster's own status codes and never a redirect hop.
func transportFor(opts httpclient.Options, wrap func(http.RoundTripper) http.RoundTripper) func(*http.Transport) (http.RoundTripper, error) {
	return func(t *http.Transport) (http.RoundTripper, error) {
		t.DisableCompression = false
		var rt http.RoundTripper = &idleConnCloser{
			RoundTripper: withSDKMiddlewares(opts, refuseRedirects(t)),
			transport:    t,
		}
		if wrap != nil {
			rt = wrap(rt)
		}
		return rt, nil
	}
}
