package plugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana-plugin-sdk-go/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withResponseLimit is a request context as the SDK hands it to a handler
// when Grafana's dataproxy.response_limit is set.
func withResponseLimit(ctx context.Context, limit int) context.Context {
	return config.WithGrafanaConfig(ctx, config.NewGrafanaCfg(map[string]string{
		config.ResponseLimit: fmt.Sprint(limit),
	}))
}

// clusterTransport is the production transport chain, as Connect installs
// it, around a bare http.Transport.
func clusterTransport(t *testing.T, wrap func(http.RoundTripper) http.RoundTripper) http.RoundTripper {
	t.Helper()
	opts := sdkOptions(backend.DataSourceInstanceSettings{
		UID: "limited", Name: "limited", Type: "hydrolix-hydrolix-datasource",
	})
	rt, err := transportFor(opts, wrap)(&http.Transport{})
	require.NoError(t, err)
	return rt
}

func TestGrafanaResponseLimitCutsTheClusterAnswerShort(t *testing.T) {
	const answer = 4 << 10
	cluster := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, answer))
	}))
	defer cluster.Close()
	client := &http.Client{Transport: clusterTransport(t, nil)}

	get := func(ctx context.Context) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cluster.URL, nil)
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		return io.ReadAll(resp.Body)
	}

	t.Run("a context without Grafana's limit reads the whole answer", func(t *testing.T) {
		// The middleware's own fallback is 0: no limit, Grafana's default.
		body, err := get(context.Background())
		require.NoError(t, err)
		assert.Len(t, body, answer)
	})

	t.Run("the limit Grafana puts on the request context cuts the body", func(t *testing.T) {
		body, err := get(withResponseLimit(context.Background(), 1<<10))
		require.Error(t, err)
		assert.True(t, errors.Is(err, httpclient.ErrResponseBodyTooLarge), "got %v", err)
		assert.Len(t, body, 1<<10, "the limit's worth of body was read before the cut")
	})

	t.Run("the panel gets the reason, with the chain underneath", func(t *testing.T) {
		// The fork reports the cut in its stream plumbing; the panel's text
		// is the reason alone, and errors.Is still reaches the SDK's error.
		_, err := get(withResponseLimit(context.Background(), 1<<10))
		require.Error(t, err)
		mutated := NewHydrolix().MutateQueryError(fmt.Errorf("readData stream: %w", err))

		assert.Equal(t, backend.ErrorSourceDownstream, mutated.ErrorSource())
		assert.True(t, strings.HasPrefix(mutated.Error(), "the cluster's answer exceeds Grafana's response limit"), mutated.Error())
		assert.True(t, errors.Is(mutated, httpclient.ErrResponseBodyTooLarge))
	})
}

func TestTransportForStillRefusesRedirects(t *testing.T) {
	// The guard sits under the SDK chain; through the fork's real client a
	// hop is still refused before any header can follow it.
	p := newRedirectingPair(t)
	db := clickhouse.OpenDB(p.options(func(rt http.RoundTripper) http.RoundTripper {
		chain, err := transportFor(sdkOptions(backend.DataSourceInstanceSettings{UID: "redirecting"}), nil)(rt.(*http.Transport))
		require.NoError(t, err)
		return chain
	}))
	defer func() { _ = db.Close() }()
	err := db.PingContext(withResponseLimit(context.Background(), 1<<10))

	require.Error(t, err)
	var redirect *RedirectError
	require.True(t, errors.As(err, &redirect), "got %v", err)
	assert.Equal(t, http.StatusTemporaryRedirect, redirect.Status)
	assert.Empty(t, p.elsewhereSaw.requests(), "a request reached the redirect target")
}

func TestTransportForReleasesTheConnectionOnClose(t *testing.T) {
	// The SDK's middlewares are RoundTripperFuncs, without CloseIdleConnections;
	// idleConnCloser puts it back on top, or the fork's httpConnect.close()
	// would release nothing (see TestRefuseRedirectsReleasesTheConnectionOnClose
	// for the witness).
	server := newSocketCloses(t)
	client := &http.Client{Transport: clusterTransport(t, nil)}
	openKeepAlive(t, client, server.URL)

	client.CloseIdleConnections()
	assert.Eventually(t, func() bool { return server.closedConns() == 1 }, 5*time.Second, 10*time.Millisecond,
		"the idle connection was not released through the SDK chain")
}

func TestSDKOptionsLabelTheDatasource(t *testing.T) {
	// The metrics middleware is silently a no-op without datasource_type.
	labels := sdkOptions(backend.DataSourceInstanceSettings{UID: "abc", Name: "prod", Type: "hydrolix-hydrolix-datasource"}).Labels
	assert.Equal(t, map[string]string{
		"datasource_uid": "abc", "datasource_name": "prod", "datasource_type": "hydrolix-hydrolix-datasource",
	}, labels)

	assert.Equal(t, "hydrolix-hydrolix-datasource",
		sdkOptions(backend.DataSourceInstanceSettings{UID: "abc"}).Labels["datasource_type"],
		"an instance without a type is labelled by the plugin ID")
}
