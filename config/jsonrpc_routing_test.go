package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetJSONRPCNodeByHeight_RecentWindow(t *testing.T) {
	SetConfig(&Config{
		Upstream: []Node{
			{JSONRPC: "https://recent.example", Blocks: []uint64{1000}},
			{JSONRPC: "http://archival.example", Blocks: []uint64{1, 50_000_000}},
			{JSONRPC: "http://tip.example", Blocks: []uint64{50_000_000, 0}},
		},
	})
	SetEVMTipLookup(func() (uint64, bool) { return 100_000_000, true })

	require.Equal(t, "https://recent.example", GetJSONRPCNodeByHeight(0).JSONRPC)
	require.Equal(t, "https://recent.example", GetJSONRPCNodeByHeight(99_999_500).JSONRPC)
	require.Equal(t, "https://recent.example", GetJSONRPCNodeByHeight(100_000_000).JSONRPC)

	require.Equal(t, "http://archival.example", GetJSONRPCNodeByHeight(1_000_000).JSONRPC)
	require.Equal(t, "http://tip.example", GetJSONRPCNodeByHeight(75_000_000).JSONRPC)
}

func TestGetJSONRPCNodeByHeight_RecentWindowWithoutTip(t *testing.T) {
	SetConfig(&Config{
		Upstream: []Node{
			{JSONRPC: "https://recent.example", Blocks: []uint64{1000}},
			{JSONRPC: "http://archival.example", Blocks: []uint64{1, 50_000_000}},
		},
	})
	SetEVMTipLookup(func() (uint64, bool) { return 0, false })

	require.Equal(t, "https://recent.example", GetJSONRPCNodeByHeight(0).JSONRPC)
	require.Equal(t, "http://archival.example", GetJSONRPCNodeByHeight(1_000_000).JSONRPC)
}

func TestGetJSONRPCNodeByHeight_FallbackToRecentWindow(t *testing.T) {
	SetConfig(&Config{
		Upstream: []Node{
			{JSONRPC: "https://recent.example", Blocks: []uint64{1000}},
			{JSONRPC: "http://archival.example", Blocks: []uint64{1, 50_000}},
		},
	})
	SetEVMTipLookup(func() (uint64, bool) { return 100_000_000, true })

	require.Equal(t, "https://recent.example", GetJSONRPCNodeByHeight(99_000).JSONRPC)
}

func TestLoadConfig_RejectsZeroRecentWindow(t *testing.T) {
	data := []byte(`
upstream:
  - jsonrpc: http://localhost:8545
    blocks: [0]
ports:
  jsonrpc: 8545
`)
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, data, 0o644))

	_, err := LoadConfig(path)
	require.ErrorContains(t, err, "recent-window size must be > 0")
}
