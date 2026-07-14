package models

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/assert"
)

func TestAdHocFilter_JSONRoundTrip(t *testing.T) {
	t.Run("single-value operator", func(t *testing.T) {
		input := []byte(`{"key":"host","operator":"=","value":"prod-1"}`)
		var f AdHocFilter
		assert.NoError(t, json.Unmarshal(input, &f))
		assert.Equal(t, "host", f.Key)
		assert.Equal(t, "=", f.Operator)
		assert.Equal(t, "prod-1", f.Value)
		assert.Empty(t, f.Values)
	})

	t.Run("multi-value operator", func(t *testing.T) {
		input := []byte(`{"key":"tier","operator":"=|","value":"","values":["a","b"]}`)
		var f AdHocFilter
		assert.NoError(t, json.Unmarshal(input, &f))
		assert.Equal(t, "tier", f.Key)
		assert.Equal(t, "=|", f.Operator)
		assert.Equal(t, []string{"a", "b"}, f.Values)
	})
}

func TestHdxQuery_JSONRoundTrip(t *testing.T) {
	input := []byte(`{"rawSql":"SELECT 1","format":1,"round":"1m","querySettings":[{"setting":"max_threads","value":"4"}],"filters":[{"key":"host","operator":"=","value":"prod-1"}],"meta":{"timezone":"UTC"}}`)
	var q HdxQuery
	assert.NoError(t, json.Unmarshal(input, &q))

	assert.Equal(t, "SELECT 1", q.RawSQL)
	assert.Equal(t, 1, q.Format)
	assert.Equal(t, "1m", q.Round)
	assert.Equal(t, []QuerySetting{{Setting: "max_threads", Value: "4"}}, q.QuerySettings)
	assert.Equal(t, []AdHocFilter{{Key: "host", Operator: "=", Value: "prod-1"}}, q.Filters)
	assert.Equal(t, "UTC", q.Meta.TimeZone)

	out, err := json.Marshal(q)
	assert.NoError(t, err)

	// Round-trip should preserve every JSON-tagged field.
	var q2 HdxQuery
	assert.NoError(t, json.Unmarshal(out, &q2))
	assert.Equal(t, q, q2)
}

func TestHdxQuery_NonJSONFieldsNotMarshalled(t *testing.T) {
	q := HdxQuery{RawSQL: "SELECT 1"}
	q.Headers = map[string][]string{"X-Test": {"value"}}

	out, err := json.Marshal(q)
	assert.NoError(t, err)
	// TimeRange, Interval, Headers carry `json:"-"`; none should appear in output.
	assert.NotContains(t, string(out), "TimeRange")
	assert.NotContains(t, string(out), "Interval")
	assert.NotContains(t, string(out), "Headers")
	assert.NotContains(t, string(out), "X-Test")
}

func TestHdxQuery_WithSQL(t *testing.T) {
	t0 := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	orig := &HdxQuery{
		RawSQL:        "SELECT $__timeFilter FROM events",
		Format:        1,
		Round:         "1m",
		QuerySettings: []QuerySetting{{Setting: "max_threads", Value: "4"}},
		Filters:       []AdHocFilter{{Key: "host", Operator: "=", Value: "prod-1"}},
		TimeRange:     backend.TimeRange{From: t0, To: t0.Add(time.Hour)},
		Interval:      30 * time.Second,
		Headers:       http.Header{"Authorization": []string{"Bearer t"}},
	}
	orig.Meta.TimeZone = "UTC"

	clone := orig.WithSQL("SELECT 1 FROM events")

	// New SQL on the clone, everything else preserved.
	assert.Equal(t, "SELECT 1 FROM events", clone.RawSQL)
	assert.Equal(t, orig.Format, clone.Format)
	assert.Equal(t, orig.Round, clone.Round)
	assert.Equal(t, orig.QuerySettings, clone.QuerySettings)
	assert.Equal(t, orig.Filters, clone.Filters)
	assert.Equal(t, orig.Meta, clone.Meta)
	assert.Equal(t, orig.TimeRange, clone.TimeRange)
	assert.Equal(t, orig.Interval, clone.Interval)
	assert.Equal(t, orig.Headers, clone.Headers)

	// Original is untouched — clone returned a copy, not an in-place rewrite.
	assert.Equal(t, "SELECT $__timeFilter FROM events", orig.RawSQL)

	// Slice/map fields share backing storage (shallow copy is intentional):
	// mutating the clone's filter slice is observable on the original. This
	// pins the documented "shallow copy" behaviour so callers can rely on it.
	clone.Filters[0].Value = "prod-2"
	assert.Equal(t, "prod-2", orig.Filters[0].Value)
}
