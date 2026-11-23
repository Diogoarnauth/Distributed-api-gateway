package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func newReverseProxy(target *url.URL) *httputil.ReverseProxy {

	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {

			r.SetURL(target)

 		},

		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {

			log.Printf("Proxy error: %v", err)

			http.Error(
				w,
				"Upstream unavailable",
				http.StatusBadGateway,
			)
		},
	}

	return proxy
}