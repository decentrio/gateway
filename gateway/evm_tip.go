package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultEVMTipCacheTTL = 2 * time.Second

var evmTipHTTPClient = &http.Client{Timeout: 5 * time.Second}

// EVMTipCache caches eth_blockNumber from a JSON-RPC upstream (the recent-window node).
type EVMTipCache struct {
	mu        sync.RWMutex
	sourceURL string
	ttl       time.Duration
	tip       uint64
	ok        bool
	fetchedAt time.Time
}

func NewEVMTipCache(sourceURL string, ttl time.Duration) *EVMTipCache {
	if ttl <= 0 {
		ttl = defaultEVMTipCacheTTL
	}
	return &EVMTipCache{
		sourceURL: sourceURL,
		ttl:       ttl,
	}
}

func (c *EVMTipCache) Tip() (uint64, bool) {
	if c == nil || c.sourceURL == "" {
		return 0, false
	}

	c.mu.RLock()
	if c.ok && time.Since(c.fetchedAt) < c.ttl {
		tip, ok := c.tip, c.ok
		c.mu.RUnlock()
		return tip, ok
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ok && time.Since(c.fetchedAt) < c.ttl {
		return c.tip, c.ok
	}

	tip, err := fetchEVMBlockNumber(c.sourceURL)
	if err != nil {
		fmt.Printf("EVM tip fetch failed (%s): %v\n", c.sourceURL, err)
		return c.tip, c.ok && time.Since(c.fetchedAt) < c.ttl*10
	}

	c.tip = tip
	c.ok = true
	c.fetchedAt = time.Now()
	return c.tip, true
}

func fetchEVMBlockNumber(jsonrpcURL string) (uint64, error) {
	payload := []byte(`{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`)
	req, err := http.NewRequest(http.MethodPost, jsonrpcURL, bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := evmTipHTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, err
	}

	var parsed struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, err
	}
	if parsed.Error != nil {
		return 0, fmt.Errorf("json-rpc error: %s", parsed.Error.Message)
	}

	return parseHexUint64(parsed.Result)
}

func parseHexUint64(hexValue string) (uint64, error) {
	hexValue = strings.TrimSpace(hexValue)
	if hexValue == "" {
		return 0, fmt.Errorf("empty block number")
	}
	hexValue = strings.TrimPrefix(hexValue, "0x")
	return strconv.ParseUint(hexValue, 16, 64)
}
