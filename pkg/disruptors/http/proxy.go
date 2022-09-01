package http

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Proxy defines the parameters used by the proxy for processing http requests and its execution state
type Proxy struct {
	// Port to listen to
	ListeningPort uint
	// Port to redirect traffic to
	TargetPort uint
	// Specification of http disruption
	HttpDisruption
	// http server that handles proxy requests
	srv *http.Server
}

// contains verifies if a list of strings contains the given string
func contains(list []string, target string) bool {
	for _, element := range list {
		if element == target {
			return true
		}
	}
	return false
}

// Start starts the execution of the proxy
func (p Proxy) Start() error {
	// define origin server URL
	originServerURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", p.TargetPort))
	if err != nil {
		return err
	}

	reverseProxy := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		statusCode := 0
		body := io.NopCloser(strings.NewReader(""))

		excluded := contains(p.Excluded, req.URL.Path)

		if !excluded && p.ErrorRate > 0 && rand.Float32() <= p.ErrorRate {
			// force error code
			statusCode = int(p.ErrorCode)
		} else {
			req.Host = originServerURL.Host
			req.URL.Host = originServerURL.Host
			req.URL.Scheme = originServerURL.Scheme
			req.RequestURI = ""
			originServerResponse, err := http.DefaultClient.Do(req)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprint(rw, err)
				return
			}

			statusCode = originServerResponse.StatusCode
			body = originServerResponse.Body
		}

		if !excluded && p.AverageDelay > 0 {
			delay := int(p.AverageDelay)
			if p.DelayVariation > 0 {
				delay = delay + int(p.DelayVariation) - 2*rand.Intn(int(p.DelayVariation))
			}
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}

		// return response to the client
		// TODO: return headers
		rw.WriteHeader(statusCode)
		io.Copy(rw, body)
	})

	p.srv = &http.Server{
		Addr:    fmt.Sprintf(":%d", p.ListeningPort),
		Handler: reverseProxy,
	}

	return p.srv.ListenAndServe()
}

// Stop stops the execution of the proxy
func (p Proxy) Stop() error {
	if p.srv != nil {
		return p.srv.Shutdown(context.Background())
	}
	return nil
}

// Force stops the proxy without waiting for connections to drain
func (p Proxy) Force() error {
	if p.srv != nil {
		return p.srv.Close()
	}
	return nil
}
