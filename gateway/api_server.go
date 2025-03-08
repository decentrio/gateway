package gateway

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	httpUtils "github.com/decentrio/gateway/http"
	"github.com/decentrio/gateway/config"
)

func Start_API_Server(server *Server) {
	fmt.Printf("Starting API server on port %d\n", server.Port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleAPIRequest)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", server.Port),
		Handler: mux,
	}

	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("Error starting API server: %v\n", err)
	}
}

func Shutdown_API_Server(server *Server) {
	fmt.Println("Shutting down API server")
	os.Exit(0)
}

func (server *Server) handleAPIRequest(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("[API] Request Path: %s\n", r.URL.Path)

	var node *config.Node
	height := r.URL.Query().Get("height")
	fmt.Println("[API] Height: ", height)

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
			fmt.Println("[API] Forwarding to Node: ", node.API)
		}
	}

	httpUtils.FowardRequest(w, r, node.API)
}
