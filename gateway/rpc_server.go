package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/cometbft/cometbft/rpc/jsonrpc/types"
	"github.com/decentrio/gateway/config"
	"github.com/decentrio/gateway/utils"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

var (
	rpcServers            = make(map[uint16]*http.Server)
	activeRPCRequestCount int32
)

func Start_RPC_Server(server *Server) {
	fmt.Printf("Starting RPC server on port %d\n", server.Port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Received RPC query: %s, method: %s\n", r.URL.Path, r.Method)
		switch r.Method {
		case "GET":
			server.handleRPCRequest(w, r)
		case "POST":
			server.handleJSONRPCRequest(w, r)
		default:
			http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		}
	},
	)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", server.Port),
		Handler: mux,
	}

	mu.Lock()
	rpcServers[server.Port] = srv
	mu.Unlock()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Printf("Failed to start RPC server: %v\n", err)
	}
}

func Shutdown_RPC_Server(server *Server) {
	mu.Lock()
	srv, exists := rpcServers[server.Port]
	if !exists {
		mu.Unlock()
		return
	}
	delete(rpcServers, server.Port)
	mu.Unlock()

	fmt.Printf("Waiting for %d active requests to complete before shutting down RPC server...\n", atomic.LoadInt32(&activeRPCRequestCount))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("All active requests completed. Proceeding with shutdown...")
	case <-ctx.Done():
		fmt.Println("[WARNING] Timeout waiting for requests. Forcing shutdown...")
	}

	if err := srv.Shutdown(ctx); err != nil {
		fmt.Printf("Error shutting down RPC server: %v\n", err)
	} else {
		fmt.Println("RPC server stopped.")
	}
}

func (server *Server) handleRPCRequest(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt32(&activeRPCRequestCount, 1)
	wg.Add(1)

	defer func() {
		wg.Done()
		atomic.AddInt32(&activeRPCRequestCount, -1)
	}()

	fmt.Printf("Received RPC query: %s\n", r.URL.Path)
	var node *config.Node

	switch r.URL.Path {
	case "/abci_info",
		"/broadcast_evidence",
		"/broadcast_tx_async",
		"/broadcast_tx_commit",
		"/broadcast_tx_sync",
		"/consensus_state",
		"/dump_consensus_state",
		"/genesis",
		"/genesis_chunked",
		"/health",
		"/net_info",
		"/num_unconfirmed_txs",
		"/status",
		"/subscribe",
		"/unsubscribe",
		"/unsubscribe_all",
		"/websocket",
		"/":
		node = config.GetNodebyHeight(0)
		if node == nil {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		} else {
			fmt.Println("Node called:", node.RPC)
		}
		httpUtils.FowardRequest(w, r, node.RPC)
		return

	case "/abci_query",
		"/block",
		"/block_results",
		"/commit",
		"/consensus_params",
		"/header",
		"/validators":
		height := r.URL.Query().Get("height")
		if height != "" {
			h, err := strconv.ParseUint(height, 10, 64)
			if err != nil {
				http.Error(w, "Invalid height", http.StatusBadRequest)
				return
			}
			node = config.GetNodebyHeight(h)
			if node == nil {
				http.Error(w, "Node not found", http.StatusNotFound)
				return
			} else {
				fmt.Println("Node called:", node.RPC)
			}
		} else {
			node = config.GetNodebyHeight(0)
			if node == nil {
				http.Error(w, "Node not found", http.StatusNotFound)
				return
			} else {
				fmt.Println("Node called:", node.RPC)
			}
		}

		httpUtils.FowardRequest(w, r, node.RPC)
		return
	case "/blockchain":
		fmt.Print(r.URL.Query())
		var height string
		if r.URL.Query().Has("maxheight") {
			height = r.URL.Query().Get("maxheight")
		} else if r.URL.Query().Has("maxHeight") {
			height = r.URL.Query().Get("maxHeight")
		} else {
			height = "0"
		}

		fmt.Println("height" + height)
		h, err := strconv.ParseUint(height, 10, 64)
		if err != nil {
			http.Error(w, "Invalid height", http.StatusBadRequest)
			return
		}
		node = config.GetNodebyHeight(h)
		if node == nil {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		} else {
			fmt.Println("Node called:", node.RPC)
		}

		httpUtils.FowardRequest(w, r, node.RPC)
		return

	case "/block_by_hash",
		"/block_search",
		"/check_tx",
		"/header_by_hash",
		"/tx",
		"/tx_search":
		RPC_nodes := config.GetNodesByType("rpc")
		var msg string = "" // msg to return to client
		for _, url := range RPC_nodes {
			res, err := httpUtils.CheckRequest(r, url)
			if err != nil {
				continue
			}
			if res == nil {
				// node did not return a response
				continue
			}
			if res.StatusCode == http.StatusOK {
				// node returned a 200 response
				fmt.Println("Node called:", url)

				for key, values := range res.Header {
					for _, value := range values {
						w.Header().Add(key, value)
					}
				}

				w.WriteHeader(res.StatusCode)
				_, err = io.Copy(w, res.Body)
				res.Body.Close()
				if err != nil {
					return
				}
				break
			} else {
				fmt.Println("Node called:", url)

				body, err := io.ReadAll(res.Body)
				res.Body.Close()
				if err != nil {
					return
				}
				msg = string(body)
				continue
			}
		}
		if msg != "" {
			// all nodes returned same non-200 response (?)
			http.Error(w, msg, http.StatusInternalServerError)
		}
		return

	default:
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}
}

func (server *Server) handleJSONRPCRequest(w http.ResponseWriter, r *http.Request) {
	var req = types.RPCRequest{}
	var res = types.RPCResponse{}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		res = types.RPCParseError(err)
		json.NewEncoder(w).Encode(res)
		return
	}

	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))

	err = req.UnmarshalJSON(body)
	if err != nil {
		res = types.RPCInvalidRequestError(req.ID, err)
		json.NewEncoder(w).Encode(res)
		return
	}

	fmt.Printf("Method: %s, Params: %s\n", req.Method, req.Params)

	var params map[string]interface{}
	err = json.Unmarshal(req.Params, &params)
	if err != nil {
		res = types.RPCInvalidParamsError(req.ID, err)
		json.NewEncoder(w).Encode(res)
		return
	}
	fmt.Println(params)

	if height, found := params["height"].(string); found {
		// handle requests that have height parameter
		if height == "" {
			height = "0"
		}
		h, err := strconv.ParseUint(height, 10, 64)
		if err != nil {
			res = types.RPCInvalidParamsError(req.ID, err)
			json.NewEncoder(w).Encode(res)
			return
		}

		fmt.Printf("Height: %d\n", h)

		node := config.GetNodebyHeight(h)
		if node == nil {
			res = types.RPCMethodNotFoundError(req.ID)
			json.NewEncoder(w).Encode(res)
			return
		}
		fmt.Println("Node called:", node.RPC)
		r.ContentLength = int64(len(body))
		httpUtils.FowardRequest(w, r, node.RPC)
		return
	} else {
		switch req.Method {
		case "block",
			"abci_info",
			"abci_query",
			"broadcast_evidence",
			"broadcast_tx_async",
			"broadcast_tx_commit",
			"broadcast_tx_sync",
			"consensus_state",
			"dump_consensus_state",
			"genesis",
			"genesis_chunked",
			"health",
			"net_info",
			"num_unconfirmed_txs",
			"status",
			"subscribe",
			"unsubscribe",
			"unsubscribe_all":
			// cases that should return latest node
			node := config.GetNodebyHeight(0)
			if node == nil {
				res = types.RPCMethodNotFoundError(req.ID)
				json.NewEncoder(w).Encode(res)
				return
			}
			fmt.Println("Node called:", node.RPC)
			r.ContentLength = int64(len(body))
			httpUtils.FowardRequest(w, r, node.RPC)
			return
		case "block_by_hash",
			"block_search",
			"check_tx",
			"header_by_hash",
			"tx",
			"tx_search":
			RPC_nodes := config.GetNodesByType("rpc")

			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				res = types.RPCInvalidRequestError(req.ID, types.RPCError{})
				json.NewEncoder(w).Encode(res)
				return
			}

			var msg string = "" // msg to return to client
			for _, url := range RPC_nodes {
				new_r := r.Clone(r.Context())
				new_r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				res, err := httpUtils.CheckRequest(new_r, url)
				if err != nil || res == nil {
					continue
				}

				if res.StatusCode == http.StatusOK {
					fmt.Println("Node called:", url)

					for key, values := range res.Header {
						for _, value := range values {
							w.Header().Add(key, value)
						}
					}

					w.WriteHeader(res.StatusCode)
					_, err = io.Copy(w, res.Body)
					res.Body.Close()
					if err != nil {
						return
					}
					break
				} else if res.StatusCode == http.StatusInternalServerError {
					fmt.Println("Node called:", url)

					body, err := io.ReadAll(res.Body)
					res.Body.Close()
					if err != nil {
						return
					}
					msg = string(body)
					continue
				} else {
					res.Body.Close()
				}
			}

			if msg != "" {
				http.Error(w, msg, http.StatusInternalServerError)
			}
			return
		case "blockchain":
			if height, found := params["maxHeight"].(string); found {
				if height == "" {
					height = "0"
				}
				h, err := strconv.ParseUint(height, 10, 64)
				if err != nil {
					res = types.RPCInvalidParamsError(req.ID, err)
					json.NewEncoder(w).Encode(res)
					return
				}

				fmt.Printf("Height: %d\n", h)

				node := config.GetNodebyHeight(h)
				if node == nil {
					res = types.RPCMethodNotFoundError(req.ID)
					json.NewEncoder(w).Encode(res)
					return
				}
				fmt.Println("Node called:", node.RPC)
				r.ContentLength = int64(len(body))
				httpUtils.FowardRequest(w, r, node.RPC)
				return
			}
		default:
			fmt.Println("Invalid method:", req.Method)
			res = types.RPCInvalidRequestError(req.ID, types.RPCError{})
			json.NewEncoder(w).Encode(res)
			return
		}
	}
}
