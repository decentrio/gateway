package httpUtils

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

var sharedTransport = &http.Transport{
	MaxIdleConns:          2000,
	MaxIdleConnsPerHost:   2000,
	IdleConnTimeout:       60 * time.Second,
	TLSHandshakeTimeout:   5 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
	ForceAttemptHTTP2:     true,
}

// init http.Client reuse globally
var httpClient = &http.Client{
	Transport: sharedTransport,
	Timeout:   0, // Không đặt timeout ở client nếu backend chậm
}

var proxyCache = sync.Map{} // map[string]*httputil.ReverseProxy

type cancelOnCloseReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnCloseReadCloser) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}
func getProxy(target *url.URL) *httputil.ReverseProxy {
	key := target.Host
	if p, ok := proxyCache.Load(key); ok {
		return p.(*httputil.ReverseProxy)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = sharedTransport
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		log.Printf("[proxy] error to %s: %v", target, err)
		http.Error(w, "Upstream error", http.StatusBadGateway)
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Set("Cache-Control", "public, max-age=10")
		return nil
	}
	proxyCache.Store(key, proxy)
	return proxy
}

func FowardRequest(w http.ResponseWriter, r *http.Request, destination string) {
	_, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	target, err := url.Parse(destination)
	if err != nil {
		http.Error(w, "Invalid target", http.StatusInternalServerError)
		return
	}
	proxy := getProxy(target)
	proxy.ServeHTTP(w, r)

}

func CheckRequest(r *http.Request, node string) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)

	new_target, err := url.Parse(node)
	if err != nil {
		cancel()
		return nil, err
	}

	new_target.Path = r.URL.Path
	new_target.RawQuery = r.URL.RawQuery

	req, err := http.NewRequestWithContext(ctx, r.Method, new_target.String(), r.Body)
	if err != nil {
		cancel()
		return nil, err
	}

	req.Header = r.Header.Clone()

	res, err := httpClient.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	res.Body = &cancelOnCloseReadCloser{ReadCloser: res.Body, cancel: cancel}
	return res, nil
}
