package gateway

import (
	"fmt"
	"log"
	"net/http"
	"os"
	// "github.com/decentrio/gateway/config"
)

func Start_JSONRPC_WS_Server(server *Server) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, you've requested: %s\n", r.URL.Path)
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", server.Port),
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting JSON-RPC WebSocket server: %v", err)
		}
	}()

	fmt.Printf("JSON-RPC WebSocket server is running on port %d\n", server.Port)
}

func Shutdown_JSONRPC_WS_Server(server *Server) {
	fmt.Println("Shutting down server")
	os.Exit(0)
}

// func (server *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
// 	fmt.Fprintf(w, "Hello, you've requested: %s\n", r.URL.Path)
// 	height := r.URL.Query().Get("height")
// 	if height != "" {
// 		fmt.Fprintf(w, "Height: %s\n", height)
// 		h, err :=
// 	}
// }
