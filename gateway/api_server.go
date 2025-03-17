package gateway

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/decentrio/gateway/config"
	"github.com/decentrio/gateway/httpUtils"
)

var (
	apiServers = make(map[uint16]*http.Server)
)

func Start_API_Server(server *Server) {
	fmt.Printf("Starting API server on port %d\n", server.Port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleAPIRequest)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", server.Port),
		Handler: mux,
	}

	mu.Lock()
	apiServers[server.Port] = srv
	mu.Unlock()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Printf("Error starting API server: %v\n", err)
	}
}

func Shutdown_API_Server(server *Server) {
	mu.Lock()
	srv, exists := apiServers[server.Port]
	if !exists {
		mu.Unlock()
		fmt.Println("API server is not running.")
		return
	}
	delete(apiServers, server.Port)
	mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		fmt.Printf("Error shutting down API server: %v\n", err)
	} else {
		fmt.Println("API server stopped.")
	}
}

func (server *Server) handleAPIRequest(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Received API query: %s\n", r.URL.Path)
	var node *config.Node
	var height uint64
	var err error
	if r.Method != "GET" && r.Method != "POST" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	} else if r.Method == "GET" {
		height_header := r.Header.Get("x-cosmos-block-height")
		if height_header != "" {
			height, err = strconv.ParseUint(height_header, 10, 64)
			if err != nil {
				http.Error(w, "Invalid height", http.StatusBadRequest)
				return
			}
		} else {
			var path = r.URL.Path
			pathSegments := strings.Split(path, "/")
			if len(pathSegments) > 0 {
				height_params := pathSegments[len(pathSegments)-1]
				if height_params == "latest" {
					height = 0
				} else {
					height, err = strconv.ParseUint(height_params, 10, 64)
					if err != nil {
						http.Error(w, "Invalid height", http.StatusBadRequest)
						return
					}
				}
			}
		}
	} else {
		height = 0
	}

	node = config.GetNodebyHeight(height)
	if node == nil {
		http.Error(w, "No node found", http.StatusNotFound)
		return
	} else {
		fmt.Println("Node: ", node.API)
	}
	httpUtils.FowardRequest(w, r, node.API)
}
