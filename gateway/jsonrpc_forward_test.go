package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/decentrio/gateway/config"
	"github.com/stretchr/testify/require"
)

func TestParseChainHeightCap(t *testing.T) {
	height, ok := parseChainHeightCap(
		"height 169311690 must be less than or equal to the current blockchain height 169311682",
	)
	require.True(t, ok)
	require.Equal(t, uint64(169_311_682), height)

	_, ok = parseChainHeightCap("some other error")
	require.False(t, ok)
}

func TestRewriteJSONRPCBlockParam(t *testing.T) {
	req := JSONRPCRequest{
		Method: "eth_getBalance",
		Params: json.RawMessage(`["0xabc","0xa1eeff"]`),
	}
	updated, err := rewriteJSONRPCBlockParam(req, 1, 169_311_682)
	require.NoError(t, err)

	var params []any
	require.NoError(t, json.Unmarshal(updated.Params, &params))
	require.Equal(t, "0xa177dc2", params[1])
}

func TestForwardJSONRPCWithBlockRetry_AdjustsBlockForLaggingBackend(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")

		if calls == 1 {
			require.Contains(t, string(body), "0xa177dca")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"height 169311690 must be less than or equal to the current blockchain height 169311682"}}`))
			return
		}

		require.Contains(t, string(body), "0xa177dc2")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x64"}`))
	}))
	defer server.Close()

	paramIndex := 1
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_getBalance",
		Params:  json.RawMessage(`["0xabc","0xa177dca"]`),
		ID:      json.RawMessage("1"),
	}
	httpReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}"))
	res, err := forwardJSONRPCWithBlockRetry(httpReq, req, server.URL, &paramIndex, 169_311_690)
	require.NoError(t, err)
	require.Nil(t, res.Error)
	require.Equal(t, "0x64", res.Result)
	require.Equal(t, 2, calls)
}

func TestForwardJSONRPCWithBlockRetry_ReroutesToRecentUpstream(t *testing.T) {
	archival := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"height 169311690 must be less than or equal to the current blockchain height 169311682"}}`))
	}))
	defer archival.Close()

	recent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.Contains(t, string(body), "0xa177dca")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x64"}`))
	}))
	defer recent.Close()

	config.SetConfig(&config.Config{
		Upstream: []config.Node{
			{JSONRPC: recent.URL, Blocks: []uint64{1000}},
			{JSONRPC: archival.URL, Blocks: []uint64{142_284_075, 0}},
		},
	})

	paramIndex := 1
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_getBalance",
		Params:  json.RawMessage(`["0xabc","0xa177dca"]`),
		ID:      json.RawMessage("1"),
	}
	httpReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}"))
	res, err := forwardJSONRPCWithBlockRetry(httpReq, req, archival.URL, &paramIndex, 169_311_690)
	require.NoError(t, err)
	require.Nil(t, res.Error)
	require.Equal(t, "0x64", res.Result)
}
