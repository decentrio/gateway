package http

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func FowardRequest(w http.ResponseWriter, r *http.Request, destination string) {
	fmt.Fprintf(w, "Hello, you've requested: %s\n", r.URL.Path)
	target, err := url.Parse(destination)
	if err != nil {
		http.Error(w, "Invalid target", http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ServeHTTP(w, r)
}