package gateway

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	// "github.com/decentrio/gateway/config"
)

func Start_JSON_RPC_Server(server *Server) {
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
			log.Fatalf("Error starting JSON-RPC server: %v", err)
		}
	}()

	fmt.Printf("JSON-RPC server is running on port %d\n", server.Port)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	fmt.Println("\nShutting down JSON-RPC server...")
	if err := srv.Close(); err != nil {
		log.Fatalf("Error shutting down JSON-RPC server: %v", err)
	}
	fmt.Println("JSON-RPC server stopped.")
}

func Shutdown_JSON_RPC_Server(server *Server) {
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
