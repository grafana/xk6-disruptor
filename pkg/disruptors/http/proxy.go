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

// Defines an interface for a proxy
type HttpProxy interface {
	Start() error
	Stop() error
	Force() error
}

// ProxyConfig specifies the configuration for the http proxy
type HttpProxyConfig struct {
	// Port on which the proxy will be running
	ListeningPort uint
}

// HttpProxyTarget defines the upstream target  to forward requests to
type HttpProxyTarget struct {
	// Port to redirect traffic to
	Port uint
}

// Proxy defines the parameters used by the proxy for processing http requests and its execution state
type proxy struct {
	config     HttpProxyConfig
	target     HttpProxyTarget
	disruption HttpDisruption
	srv        *http.Server
}

// NewProxy return a new HttpProxy
func NewHttpProxy(target HttpProxyTarget, disruption HttpDisruption, config HttpProxyConfig) (HttpProxy, error) {
	if config.ListeningPort == 0 {
		return nil, fmt.Errorf("proxy's listening port must be valid tcp port")
	}

	if target.Port == config.ListeningPort {
		return nil, fmt.Errorf("target port and listening port cannot be the same")
	}

	return &proxy{
		target:     target,
		disruption: disruption,
		config:     config,
	}, nil
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
func (p *proxy) Start() error {
	originServerURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", p.target.Port))
	if err != nil {
		return err
	}

	reverseProxy := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		statusCode := 0
		body := io.NopCloser(strings.NewReader(""))

		excluded := contains(p.disruption.Excluded, req.URL.Path)

		if !excluded && p.disruption.ErrorRate > 0 && rand.Float32() <= p.disruption.ErrorRate {
			// force error code
			statusCode = int(p.disruption.ErrorCode)
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

		if !excluded && p.disruption.AverageDelay > 0 {
			delay := int(p.disruption.AverageDelay)
			if p.disruption.DelayVariation > 0 {
				delay = delay + int(p.disruption.DelayVariation) - 2*rand.Intn(int(p.disruption.DelayVariation))
			}
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}

		// return response to the client
		// TODO: return headers
		rw.WriteHeader(statusCode)
		io.Copy(rw, body)
	})

	p.srv = &http.Server{
		Addr:    fmt.Sprintf(":%d", p.config.ListeningPort),
		Handler: reverseProxy,
	}

	return p.srv.ListenAndServe()
}

// Stop stops the execution of the proxy
func (p *proxy) Stop() error {
	if p.srv != nil {
		return p.srv.Shutdown(context.Background())
	}
	return nil
}

// Force stops the proxy without waiting for connections to drain
func (p *proxy) Force() error {
	if p.srv != nil {
		return p.srv.Close()
	}
	return nil
}
