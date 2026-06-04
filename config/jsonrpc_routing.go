package config

// EVMTipLookup returns the cached EVM chain tip block number and whether it is valid.
// Set by the gateway at startup from the first recent-window upstream's jsonrpc URL.
type EVMTipLookup func() (tip uint64, ok bool)

var evmTipLookup EVMTipLookup

// SetEVMTipLookup configures the EVM tip provider used for JSON-RPC recent-window routing.
func SetEVMTipLookup(fn EVMTipLookup) {
	evmTipLookup = fn
}

// FirstRecentWindowNode returns the first upstream with blocks: [N] (recent-window semantics).
func FirstRecentWindowNode() *Node {
	if cfg == nil {
		return nil
	}
	for i := range cfg.Upstream {
		if isRecentWindowNode(&cfg.Upstream[i]) {
			return &cfg.Upstream[i]
		}
	}
	return nil
}

func isRecentWindowNode(n *Node) bool {
	return len(n.Blocks) == 1 && n.Blocks[0] > 0
}

func recentWindowSize(n *Node) uint64 {
	if !isRecentWindowNode(n) {
		return 0
	}
	return n.Blocks[0]
}

func matchesRecentWindow(n *Node, height uint64, tip uint64, tipOK bool) bool {
	if !isRecentWindowNode(n) {
		return false
	}
	if height == 0 {
		return true
	}
	if !tipOK {
		return false
	}
	window := recentWindowSize(n)
	// Cached tip can lag behind a block number the client just received from
	// eth_blockNumber on the recent upstream. Allow heights up to window blocks
	// above the cached tip so back-to-back calls stay on the recent node.
	effectiveTip := tip
	if height > tip && height-tip <= window {
		effectiveTip = height
	}
	if height > effectiveTip {
		return false
	}
	lowerBound := uint64(0)
	if effectiveTip > window {
		lowerBound = effectiveTip - window
	}
	return height >= lowerBound
}

func matchesStaticRange(n *Node, height uint64, tip uint64, tipOK bool) bool {
	if len(n.Blocks) != 2 || height == 0 {
		return false
	}
	if n.Blocks[1] != 0 {
		return height >= n.Blocks[0] && height <= n.Blocks[1]
	}
	if height < n.Blocks[0] {
		return false
	}
	// Open range [x, 0] only covers blocks the upstream actually has.
	if tipOK && height > tip {
		return false
	}
	return true
}

func firstJSONRPCNodeMatching(match func(*Node) bool) *Node {
	if cfg == nil {
		return nil
	}
	for i := range cfg.Upstream {
		if match(&cfg.Upstream[i]) {
			return &cfg.Upstream[i]
		}
	}
	return nil
}

// GetJSONRPCNodeByHeight selects an upstream for EVM JSON-RPC by block height.
//
// Routing order (first match in upstream list):
//   - blocks: [N]     — height 0 (latest) or height within the last N blocks of cached EVM tip
//   - blocks: [x, y]  — static closed range (y != 0)
//   - blocks: [x, 0]  — static open range from x through tip
//
// Fallback: first recent-window node, then first [x, 0] node.
func GetJSONRPCNodeByHeight(height uint64) *Node {
	var tip uint64
	var tipOK bool
	if evmTipLookup != nil {
		tip, tipOK = evmTipLookup()
	}

	if node := firstJSONRPCNodeMatching(func(n *Node) bool {
		return matchesRecentWindow(n, height, tip, tipOK)
	}); node != nil {
		return node
	}

	if height > 0 {
		if node := firstJSONRPCNodeMatching(func(n *Node) bool {
			return matchesStaticRange(n, height, tip, tipOK)
		}); node != nil {
			return node
		}
	}

	if node := FirstRecentWindowNode(); node != nil {
		return node
	}

	if cfg != nil {
		for i := range cfg.Upstream {
			n := &cfg.Upstream[i]
			if len(n.Blocks) == 2 && n.Blocks[1] == 0 {
				return n
			}
		}
	}

	return nil
}
