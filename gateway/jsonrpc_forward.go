package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/decentrio/gateway/config"
)

// Matches upstream errors such as:
// "height 169311690 must be less than or equal to the current blockchain height 169311682"
var chainHeightCapErrorRE = regexp.MustCompile(`(?i)must be less than or equal to the current blockchain height\s+(\d+)`)

func parseChainHeightCap(message string) (uint64, bool) {
	m := chainHeightCapErrorRE.FindStringSubmatch(message)
	if len(m) != 2 {
		return 0, false
	}
	height, err := strconv.ParseUint(m[1], 10, 64)
	return height, err == nil
}

func rewriteJSONRPCBlockParam(req JSONRPCRequest, paramIndex int, height uint64) (JSONRPCRequest, error) {
	var params []any
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return req, err
	}
	if len(params) <= paramIndex {
		return req, fmt.Errorf("block param index %d out of range", paramIndex)
	}

	useHex := true
	if s, ok := params[paramIndex].(string); ok && !strings.HasPrefix(s, "0x") {
		if _, err := strconv.ParseUint(s, 10, 64); err == nil {
			useHex = false
		}
	}
	if useHex {
		params[paramIndex] = fmt.Sprintf("0x%x", height)
	} else {
		params[paramIndex] = fmt.Sprintf("%d", height)
	}

	encoded, err := json.Marshal(params)
	if err != nil {
		return req, err
	}
	req.Params = encoded
	return req, nil
}

// forwardJSONRPCWithBlockRetry forwards a height-based JSON-RPC request.
//
// With IP stickiness on the recent upstream LB, eth_blockNumber and follow-up calls
// from the gateway land on the same backend. Block-ahead errors therefore usually
// mean the request was misrouted to a lagging archival node (stale tip cache).
// On that error we first retry on the recent-window upstream at the original block,
// then fall back to the chain height the rejecting node reported.
func forwardJSONRPCWithBlockRetry(
	r *http.Request,
	req JSONRPCRequest,
	destination string,
	heightParamIndex *int,
	requestedHeight uint64,
) (JSONRPCResponse, error) {
	currentReq := req
	currentHeight := requestedHeight
	currentDestination := destination
	retriedRecent := false
	var last JSONRPCResponse

	for attempt := 0; attempt < 3; attempt++ {
		reqBody, err := json.Marshal(currentReq)
		if err != nil {
			return JSONRPCResponse{}, err
		}
		forwardReq := r.Clone(r.Context())
		forwardReq.Body = io.NopCloser(bytes.NewReader(reqBody))
		forwardReq.Header.Set("Content-Type", "application/json")
		forwardReq.ContentLength = int64(len(reqBody))

		res, err := forwardRequestAndGetResponse(forwardReq, currentDestination)
		if err != nil {
			return JSONRPCResponse{}, err
		}
		last = res
		if res.Error == nil {
			return res, nil
		}

		chainHeight, ok := parseChainHeightCap(res.Error.Message)
		if !ok || heightParamIndex == nil || currentHeight <= chainHeight {
			return res, nil
		}

		recent := config.FirstRecentWindowNode()
		if !retriedRecent && recent != nil && recent.JSONRPC != "" &&
			currentDestination != recent.JSONRPC {
			fmt.Printf(
				"Block-ahead error from %s (requested=%d chain=%d), retrying recent upstream %s\n",
				currentDestination, currentHeight, chainHeight, recent.JSONRPC,
			)
			currentDestination = recent.JSONRPC
			currentReq = req
			currentHeight = requestedHeight
			retriedRecent = true
			continue
		}

		fmt.Printf(
			"Block-ahead error from %s (requested=%d chain=%d attempt=%d), retrying at chain height\n",
			currentDestination, currentHeight, chainHeight, attempt+1,
		)
		currentReq, err = rewriteJSONRPCBlockParam(currentReq, *heightParamIndex, chainHeight)
		if err != nil {
			return res, nil
		}
		currentHeight = chainHeight
	}

	return last, nil
}
