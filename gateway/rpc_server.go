package gateway

import (
	"fmt"
	"net/http"
	"os"

	// "github.com/decentrio/gateway/config"
)

func Start_RPC_Server(server *Server) {
	fmt.Printf("Starting server on port %d\n", server.Port)
	// http.HandleFunc("/", server.handleRequest)
	http.HandleFunc("/",
		func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Hello, you've requested: %s\n", r.URL.Path)
		})
	err := http.ListenAndServe(fmt.Sprintf(":%d", server.Port), nil)
	if err != nil {
		fmt.Printf("Error starting server: %v\n", err)
	}
}

func Shutdown_RPC_Server(server *Server) {
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