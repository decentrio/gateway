package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEVMTipCache_FetchesAndCachesTip(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x64"}`))
	}))
	defer server.Close()

	cache := NewEVMTipCache(server.URL, time.Minute)
	tip, ok := cache.Tip()
	require.True(t, ok)
	require.Equal(t, uint64(100), tip)

	tip, ok = cache.Tip()
	require.True(t, ok)
	require.Equal(t, uint64(100), tip)
	require.Equal(t, 1, calls)
}

func TestParseHexUint64(t *testing.T) {
	value, err := parseHexUint64("0x2a")
	require.NoError(t, err)
	require.Equal(t, uint64(42), value)
}
