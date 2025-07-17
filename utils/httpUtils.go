package httpUtils

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"runtime"
	"time"
)

var sharedTransport = &http.Transport{
	MaxIdleConns:        200,
	MaxIdleConnsPerHost: 100,
	IdleConnTimeout:     90 * time.Second,
	TLSHandshakeTimeout: 10 * time.Second,
}

// init http.Client reuse globally
var httpClient = &http.Client{
	Transport: sharedTransport,
	Timeout:   15 * time.Second,
}

var (
	semaphore = make(chan struct{}, 8)
)

func FowardRequest(w http.ResponseWriter, r *http.Request, destination string) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	select {
	case semaphore <- struct{}{}:
		defer func() { <-semaphore }()
	case <-ctx.Done():
		http.Error(w, "Server busy, please try again later", http.StatusTooManyRequests)
		return
	}
	target, err := url.Parse(destination)
	if err != nil {
		http.Error(w, "Invalid target", http.StatusInternalServerError)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = sharedTransport
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
	}

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

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	log.Println("Active connections:", runtime.NumGoroutine())
	return res, nil
}
