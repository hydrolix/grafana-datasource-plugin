package models

import (
	"encoding/json"
	"testing"

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
