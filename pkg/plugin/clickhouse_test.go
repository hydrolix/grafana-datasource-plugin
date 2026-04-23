package plugin_test

import (
	"context"
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	hdxbuild "github.com/hydrolix/plugin/pkg/build"
	"github.com/hydrolix/plugin/pkg/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClickHouseQuery(t *testing.T) {
	ctx := context.Background()

	opts := &clickhouse.Options{
		Addr: []string{"qe-innovations-3.hydrolix.dev:443"},
		Auth: clickhouse.Auth{
			Username: "vkohut+test@hydrolix.io",
			Password: "****",
		},
		ClientInfo: clickhouse.ClientInfo{
			Products: getClientInfoProducts(ctx),
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		Protocol:    clickhouse.HTTP,
		HttpUrlPath: "/query",
		TLS:         &tls.Config{},

		BlockBufferSize: 2,
		// Connection-level: only hdx_query_output_format (matching driver.go),
		// hdx_query_streaming_result will be applied per-query via context.
		Settings: map[string]any{"hdx_query_output_format": "Native"},
		TransportFunc: func(t *http.Transport) (http.RoundTripper, error) {
			t.DisableCompression = true
			return &metadataStrippingTransport{base: t}, nil
		},
	}

	// Use clickhouse.OpenDB() to get *sql.DB — matching production path in driver.go
	db := clickhouse.OpenDB(opts)
	defer db.Close()

	// require.NoError(t, db.PingContext(ctx))

	// Apply hdx_query_streaming_result via context — matching how MutateQuery does it (driver.go lines 316-322)
	ctx = clickhouse.Context(ctx, clickhouse.WithSettings(map[string]any{
		// "hdx_query_streaming_result": clickhouse.CustomSetting{Value: "1"},
		"hdx_query_streaming_result": "1",
	}))

	query := "SELECT count() from hydro.logs where timestamp >= toDateTime(1776698895) AND timestamp <= toDateTime(1776699195)"

	var count uint64
	err := db.QueryRowContext(ctx, query).Scan(&count)
	require.NoError(t, err)
	assert.NotEmpty(t, count)
	t.Logf("row count: %d", count)
}

type metadataStrippingTransport struct {
	base http.RoundTripper
}

func (t *metadataStrippingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := t.base.RoundTrip(req)
	resp.Header.Del("Content-Encoding")
	resp.Header.Del("Content-Length")
	resp.ContentLength = -1
	if err != nil {
		return resp, err
	}
	resp.Body = plugin.NewStatsStrippingReader(resp.Body)
	return resp, nil
}

func getClientInfoProducts(ctx context.Context) (products []struct{ Name, Version string }) {
	version := backend.UserAgentFromContext(ctx).GrafanaVersion()

	if version != "" {
		products = append(products, struct{ Name, Version string }{
			Name:    "grafana",
			Version: version,
		})
	}

	info := hdxbuild.BuildInfo{}.GetBuildInfo()
	products = append(products, struct{ Name, Version string }{
		Name:    info.PluginID,
		Version: info.Version,
	})

	return products
}

// TestClickHouseQueryConnectionLevel tests with hdx_query_streaming_result at connection level
// (as currently configured in driver.go lines 132, 164).
func TestClickHouseQueryConnectionLevel(t *testing.T) {
	ctx := context.Background()

	opts := &clickhouse.Options{
		Addr: []string{"qe-innovations-3.hydrolix.dev:443"},
		Auth: clickhouse.Auth{
			Username: "vkohut+test@hydrolix.io",
			Password: "DevTest83",
		},
		ClientInfo: clickhouse.ClientInfo{
			Products: getClientInfoProducts(ctx),
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionNone,
		},
		Protocol:    clickhouse.HTTP,
		HttpUrlPath: "/query",
		TLS:         &tls.Config{},

		BlockBufferSize: 2,
		// Both settings at connection level — matching current driver.go
		Settings: map[string]any{
			"hdx_query_output_format":    "Native",
			"hdx_query_streaming_result": "1",
		},
		TransportFunc: func(t *http.Transport) (http.RoundTripper, error) {
			t.DisableCompression = false
			return &metadataStrippingTransport{base: t}, nil
		},
	}

	db := clickhouse.OpenDB(opts)
	defer db.Close()

	require.NoError(t, db.PingContext(ctx))

	query := "SELECT count() from hydro.logs where timestamp >= toDateTime(1776698895) AND timestamp <= toDateTime(1776699195)"

	var count uint64
	err := db.QueryRowContext(ctx, query).Scan(&count)
	require.NoError(t, err)
	assert.NotEmpty(t, count)
	t.Logf("row count: %d", count)
}

// TestClickHouseQueryBothLevels tests with hdx_query_streaming_result at BOTH connection and context level,
// to check if applying it twice causes issues.
func TestClickHouseQueryBothLevels(t *testing.T) {
	ctx := context.Background()

	opts := &clickhouse.Options{
		Addr: []string{"qe-innovations-3.hydrolix.dev:443"},
		Auth: clickhouse.Auth{
			Username: "vkohut+test@hydrolix.io",
			Password: "DevTest83",
		},
		ClientInfo: clickhouse.ClientInfo{
			Products: getClientInfoProducts(ctx),
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionNone,
		},
		Protocol:    clickhouse.HTTP,
		HttpUrlPath: "/query",
		TLS:         &tls.Config{},

		BlockBufferSize: 2,
		Settings: map[string]any{
			"hdx_query_output_format":    "Native",
			"hdx_query_streaming_result": "1",
		},
		TransportFunc: func(t *http.Transport) (http.RoundTripper, error) {
			t.DisableCompression = false
			return &metadataStrippingTransport{base: t}, nil
		},
	}

	db := clickhouse.OpenDB(opts)
	defer db.Close()

	require.NoError(t, db.PingContext(ctx))

	// Also apply via context (simulating what MutateQuery does when querySettings include it)
	ctx = clickhouse.Context(ctx, clickhouse.WithSettings(map[string]any{
		"hdx_query_streaming_result": clickhouse.CustomSetting{Value: "1"},
	}))

	query := "SELECT count() from hydro.logs where timestamp >= toDateTime(1776698895) AND timestamp <= toDateTime(1776699195)"

	var count uint64
	err := db.QueryRowContext(ctx, query).Scan(&count)
	require.NoError(t, err)
	assert.NotEmpty(t, count)
	t.Logf("row count: %d", count)
}
