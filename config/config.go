package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Node struct {
	RPC        string   `yaml:"rpc"`
	API        string   `yaml:"api"`
	GRPC       string   `yaml:"grpc"`
	JSONRPC    string   `yaml:"jsonrpc"`
	JSONRPC_WS string   `yaml:"jsonrpc_ws"`
	Blocks     []uint64 `yaml:"blocks"`
}

type Ports struct {
	RPC        uint16 `yaml:"rpc"`
	GRPC       uint16 `yaml:"grpc"`
	API        uint16 `yaml:"api"`
	JSONRPC    uint16 `yaml:"jsonrpc"`
	JSONRPC_WS uint16 `yaml:"jsonrpc_ws"`
}

// MethodRoutingEntry defines how to route a JSON-RPC method (eth_*, debug_*, etc.).
// Exactly one of HeightFromParam, HashBased, or NotSupported should be set for explicit routing.
// Gated methods require the feature flag (--enable-debug for debug_* methods).
type MethodRoutingEntry struct {
	HeightFromParam *int  `yaml:"height_from_param"` // 0, 1, or 2 - block number/tag at this param index
	HashBased       bool  `yaml:"hash_based"`        // try all nodes (block/tx hash in params)
	NotSupported    bool  `yaml:"not_supported"`      // return "Method not supported"
	Gated           bool  `yaml:"gated"`             // requires feature flag (e.g. for debug_ methods)
}

type Config struct {
	Upstream      []Node                       `yaml:"upstream"`
	Ports         Ports                        `yaml:"ports"`
	MethodRouting map[string]MethodRoutingEntry `yaml:"method_routing"`
}

var DefaultConfig = Config{
	Upstream: []Node{
		{
			RPC:        "http://localhost:26657",
			API:        "http://localhost:1317",
			GRPC:       "localhost:9090",
			JSONRPC:    "http://localhost:8545",
			JSONRPC_WS: "http://localhost:8546/websocket",
			Blocks:     []uint64{1, 1000},
		},
	},
	Ports: Ports{
		RPC:        26657,
		GRPC:       9090,
		API:        1317,
		JSONRPC:    8545,
		JSONRPC_WS: 8546,
	},
}

var cfg *Config

// defaultMethodRouting is used when config has no method_routing or method is not in config.
// Param indices: 0 = block number in first param, 1 = second param (e.g. eth_call), 2 = third param (e.g. eth_getStorageAt).
var defaultMethodRouting map[string]MethodRoutingEntry

func init() {
	param0 := 0
	param1 := 1
	param2 := 2
	defaultMethodRouting = map[string]MethodRoutingEntry{
		// height_from_param 1
		"eth_getBalance":          {HeightFromParam: &param1},
		"eth_getTransactionCount": {HeightFromParam: &param1},
		"eth_getCode":             {HeightFromParam: &param1},
		"eth_call":                {HeightFromParam: &param1},
		// height_from_param 2
		"eth_getStorageAt": {HeightFromParam: &param2},
		// height_from_param 0
		"eth_getBlockTransactionCountByNumber": {HeightFromParam: &param0},
		"eth_getBlockByNumber":                 {HeightFromParam: &param0},
		"eth_getBlockReceipts":                  {HeightFromParam: &param0},
		"eth_getTransactionByBlockNumberAndIndex": {HeightFromParam: &param0},
		"eth_getUncleByBlockNumberAndIndex":    {HeightFromParam: &param0},
		// hash_based
		"eth_getTransactionByHash":              {HashBased: true},
		"eth_getTransactionReceipt":             {HashBased: true},
		"eth_getBlockByHash":                    {HashBased: true},
		"eth_getBlockTransactionCountByHash":    {HashBased: true},
		"eth_getTransactionByBlockHashAndIndex": {HashBased: true},
		"eth_getUncleByBlockHashAndIndex":       {HashBased: true},
		// not_supported
		"eth_newFilter": {NotSupported: true},
		"eth_getLogs":   {NotSupported: true}, // single-request path uses custom filter logic in code
		// debug_ (gated) - block number at param 0
		"debug_traceBlockByNumber": {HeightFromParam: &param0, Gated: true},
		"debug_getBlockRlp":        {HeightFromParam: &param0, Gated: true},
		"debug_printBlock":         {HeightFromParam: &param0, Gated: true},
		"debug_seedHash":           {HeightFromParam: &param0, Gated: true},
		"debug_dumpBlock":          {HeightFromParam: &param0, Gated: true},
		// debug_ - block number at param 1
		"debug_traceCall": {HeightFromParam: &param1, Gated: true},
		// debug_ - hash_based (block or tx hash in params)
		"debug_traceBlockByHash":   {HashBased: true, Gated: true},
		"debug_traceTransaction":  {HashBased: true, Gated: true},
		"debug_storageRangeAt":    {HashBased: true, Gated: true},
		"debug_accountRange":      {HashBased: true, Gated: true},
		"debug_traceBadBlock":     {HashBased: true, Gated: true},
		// debug_ - no block param (forward to default node)
		"debug_metrics":   {Gated: true},
		"debug_memStats":  {Gated: true},
		"debug_gcStats":   {Gated: true},
		"debug_cpuProfile": {Gated: true},
	}
}

// GetMethodRouting returns the effective routing for a method (from config file, then built-in default).
func GetMethodRouting(method string) MethodRoutingEntry {
	if cfg != nil && cfg.MethodRouting != nil {
		if e, ok := cfg.MethodRouting[method]; ok {
			return e
		}
	}
	// Fallback when config has no method_routing or method not in config (e.g. partial override).
	if e, ok := defaultMethodRouting[method]; ok {
		return e
	}
	return MethodRoutingEntry{}
}

func GenerateConfig() error {
	cfg := DefaultConfig
	cfg.MethodRouting = defaultMethodRouting
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %w", err)
	}
	err = os.WriteFile("config.yaml", data, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to write default config: %w", err)
	}
	return nil
}

func LoadConfig(configPath string) (*Config, error) {
	config := &Config{}
	file, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	err = yaml.Unmarshal(file, config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// If config has no method_routing, use built-in default so routing is read from config struct.
	if config.MethodRouting == nil {
		config.MethodRouting = defaultMethodRouting
	}

	for i, node := range config.Upstream {
		if len(node.Blocks) > 2 {
			return nil, fmt.Errorf("invalid blocks range for node %d", i+1)
		}
		if len(node.Blocks) == 1 && node.Blocks[0] == 0 {
			return nil, fmt.Errorf("invalid blocks for node %d: recent-window size must be > 0", i+1)
		}
	}

	if config.MethodRouting != nil {
		for method, e := range config.MethodRouting {
			if e.HeightFromParam != nil && (*e.HeightFromParam < 0 || *e.HeightFromParam > 2) {
				return nil, fmt.Errorf("method_routing: %q height_from_param must be 0, 1, or 2", method)
			}
		}
	}

	return config, nil
}

func GetConfig() *Config {
	return cfg
}

func SetConfig(config *Config) {
	cfg = config
}

func GetNodebyHeight(height uint64) *Node {
	if height == 0 {
		fmt.Println("find node for height is zero")

		// prioritize [x] node
		for _, n := range cfg.Upstream {
			if len(n.Blocks) == 1 {
				return &n
			}
		}

		// fallback: If no pruned nodes found, return [x, 0] node.
		for _, n := range cfg.Upstream {
			if len(n.Blocks) == 2 && n.Blocks[1] == 0 {
				return &n
			}
		}
	} else {
		fmt.Println("find node for height ", height)

		// prioritize [x, y] node
		// for [x,y] nodes, if height is between x and y, return that node.
		// for [x,0] nodes, if height is greater than x, return that node.
		for _, n := range cfg.Upstream {
			if len(n.Blocks) == 2 {
				if n.Blocks[1] != 0 {
					if height >= n.Blocks[0] && height <= n.Blocks[1] {
						return &n
					}
				} else if height >= n.Blocks[0] {
					return &n
				}
			}
		}

		// fallback: If no nodes found for the given height, return pruned node.
		for _, n := range cfg.Upstream {
			if len(n.Blocks) == 1 {
				return &n
			}
		}
	}

	return nil
}

func GetNodesByType(nodeType string) []string {
	nodes := []string{}
	for _, node := range cfg.Upstream {
		switch nodeType {
		case "rpc":
			nodes = append(nodes, node.RPC)
		case "api":
			nodes = append(nodes, node.API)
		case "grpc":
			nodes = append(nodes, node.GRPC)
		case "jsonrpc":
			nodes = append(nodes, node.JSONRPC)
		case "jsonrpc_ws":
			nodes = append(nodes, node.JSONRPC_WS)
		}
	}
	return nodes
}
