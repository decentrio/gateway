package httpUtils

import (
	// "fmt"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func FowardRequest(w http.ResponseWriter, r *http.Request, destination string) {
	target, err := url.Parse(destination)
	if err != nil {
		http.Error(w, "Invalid target", http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ServeHTTP(w, r)
}

func CheckRequest(w http.ResponseWriter, r *http.Request, node string) bool {
	new_target, err := url.Parse(node)
	if err != nil {
		http.Error(w, "Invalid target", http.StatusInternalServerError)
		return false
	}
	new_target.Path = r.URL.Path
	new_target.RawQuery = r.URL.RawQuery

	req, err := http.NewRequest(r.Method, new_target.String(), r.Body)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusInternalServerError)
		return false
	}

	req.Header = r.Header.Clone()

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Invalid response", http.StatusInternalServerError)
		return false
	}

	defer res.Body.Close()
	fmt.Printf(res.Status)
	// if res.StatusCode != http.StatusOK {
	// 	fmt.Printf("Node %s with status code %d \n", node, res.StatusCode)
	// 	return false
	// }

	for key, values := range res.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(res.StatusCode)
	_, err = io.Copy(w, res.Body)
	if err != nil {
		http.Error(w, "Invalid response", http.StatusInternalServerError)
		return false
	}
	return true
}