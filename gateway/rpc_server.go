package gateway

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	httpUtils "github.com/decentrio/gateway/http"

	"github.com/decentrio/gateway/config"
	// "github.com/decentrio/gateway/config"
)

func Start_RPC_Server(server *Server) {
	fmt.Printf("Starting server on port %d\n", server.Port)
	http.HandleFunc("/", server.handleRequest)
	err := http.ListenAndServe(fmt.Sprintf(":%d", server.Port), nil)
	if err != nil {
		fmt.Printf("Error starting server: %v\n", err)
	}
}

func Shutdown_RPC_Server(server *Server) {
	fmt.Println("Shutting down server")
	os.Exit(0)
}

func (server *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("%s\n", r.URL.Path)
	
	var node *config.Node
	height := r.URL.Query().Get("height")
	fmt.Println("Height: ", height)

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
			fmt.Println("Node: ", node.RPC)
		}
	} else {

	}

	httpUtils.FowardRequest(w, r, node.RPC)
}