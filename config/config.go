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

type Config struct {
	Upstream []Node `yaml:"upstream"`
	Ports    Ports  `yaml:"ports"`
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

func GenerateConfig() error {
	data, err := yaml.Marshal(DefaultConfig)
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

	for i, node := range config.Upstream {
		if len(node.Blocks) > 2 {
			return nil, fmt.Errorf("invalid blocks range for node %d", i+1)
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
