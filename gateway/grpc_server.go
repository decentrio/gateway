package gateway

import (
	"fmt"
	"net/http"
	"os"

	// "google.golang.org/grpc"
	// "github.com/decentrio/gateway/config"
)

func Start_GRPC_Server(server *Server) {
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

func Shutdown_GRPC_Server(server *Server) {
	fmt.Println("Shutting down server")
	os.Exit(0)
}