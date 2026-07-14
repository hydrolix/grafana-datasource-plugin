package plugin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInjectConnectionArgs_AddsToBodyWithoutOne(t *testing.T) {
	body := json.RawMessage(`{"rawSql":"SELECT 1"}`)
	out, err := injectConnectionArgs(body, map[string]string{"oauthToken": "t"})
	require.NoError(t, err)

	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out, &got))

	assert.JSONEq(t, `"SELECT 1"`, string(got["rawSql"]))
	assert.JSONEq(t, `{"oauthToken":"t"}`, string(got["connectionArgs"]))
}

func TestInjectConnectionArgs_OverwritesExisting(t *testing.T) {
	body := json.RawMessage(`{"rawSql":"X","connectionArgs":{"stale":"v","oauthToken":"old"}}`)
	out, err := injectConnectionArgs(body, map[string]string{"oauthToken": "new"})
	require.NoError(t, err)

	var got struct {
		RawSql         string            `json:"rawSql"`
		ConnectionArgs map[string]string `json:"connectionArgs"`
	}
	require.NoError(t, json.Unmarshal(out, &got))

	assert.Equal(t, "X", got.RawSql)
	assert.Equal(t, map[string]string{"oauthToken": "new"}, got.ConnectionArgs)
}

func TestInjectConnectionArgs_NilBodyTreatedAsEmpty(t *testing.T) {
	out, err := injectConnectionArgs(nil, map[string]string{"orgId": "5"})
	require.NoError(t, err)

	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out, &got))

	assert.JSONEq(t, `{"orgId":"5"}`, string(got["connectionArgs"]))
	_, hasOther := got["rawSql"]
	assert.False(t, hasOther)
}

func TestInjectConnectionArgs_MalformedBodyPropagatesError(t *testing.T) {
	_, err := injectConnectionArgs(json.RawMessage("not json"), map[string]string{"oauthToken": "t"})
	assert.Error(t, err)
}

func TestInjectConnectionArgs_PreservesOtherFields(t *testing.T) {
	body := json.RawMessage(`{"rawSql":"SELECT 1","querySettings":[{"setting":"k","value":"v"}],"refId":"A"}`)
	out, err := injectConnectionArgs(body, map[string]string{"oauthToken": "t", "orgId": "5"})
	require.NoError(t, err)

	var got struct {
		RawSql         string            `json:"rawSql"`
		QuerySettings  []map[string]any  `json:"querySettings"`
		RefId          string            `json:"refId"`
		ConnectionArgs map[string]string `json:"connectionArgs"`
	}
	require.NoError(t, json.Unmarshal(out, &got))

	assert.Equal(t, "SELECT 1", got.RawSql)
	assert.Equal(t, "A", got.RefId)
	assert.Len(t, got.QuerySettings, 1)
	assert.Equal(t, "t", got.ConnectionArgs["oauthToken"])
	assert.Equal(t, "5", got.ConnectionArgs["orgId"])
}
