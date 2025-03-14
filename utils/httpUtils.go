package httpUtils

import (
	// "io"
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

func CheckRequest(r *http.Request, node string) (*http.Response, error) {
	new_target, err := url.Parse(node)
	if err != nil {
		return nil, err
	}

	new_target.Path = r.URL.Path
	new_target.RawQuery = r.URL.RawQuery

	req, err := http.NewRequest(r.Method, new_target.String(), r.Body)
	if err != nil {
		return nil, err
	}

	req.Header = r.Header.Clone()

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}