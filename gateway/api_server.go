package gateway

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"slices"

	"github.com/decentrio/gateway/config"
	"github.com/decentrio/gateway/utils"
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
	fmt.Printf("Received API query: %s\n", r.URL.Path)
	var node *config.Node
	var height uint64 = 0
	var err error
	if r.Method != "GET" && r.Method != "POST" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	} else if r.Method == "GET" {
		height_header := r.Header.Get("x-cosmos-block-height")
		if height_header != "" {
			height, err = strconv.ParseUint(height_header, 10, 64)
			if err != nil {
				http.Error(w, fmt.Sprintf("{\"code\":3,\"message\":\"type mismatch, parameter: height, error: strconv.ParseInt: parsing \\\"%s\\\": invalid syntax\",\"details\":[]}", height_header), http.StatusBadRequest)
				return
			}
		} else {
			var path = r.URL.Path
			pathSegments := strings.Split(path, "/")
			path_with_height_params := []string{"block", "blocks", "validatorsets", "historical_info"}
			if len(pathSegments) > 0 {
				height_params := pathSegments[len(pathSegments)-1]
				if slices.Contains(path_with_height_params, pathSegments[len(pathSegments)-2]) {
					height, err = strconv.ParseUint(height_params, 10, 64)
					if err != nil {
						http.Error(w, fmt.Sprintf("{\"code\":3,\"message\":\"type mismatch, parameter: height, error: strconv.ParseInt: parsing \\\"%s\\\": invalid syntax\",\"details\":[]}", height_params), http.StatusBadRequest)
						return
					}
				}
			}
		}
	}

	node = config.GetNodebyHeight(height)
	if node == nil {
		http.Error(w, "No node found", http.StatusNotFound)
		return
	} else {
		fmt.Println("Node called: ", node.API)
	}
	httpUtils.FowardRequest(w, r, node.API)
}