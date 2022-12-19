package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Proxy defines an interface for a proxy
type Proxy interface {
	Start() error
	Stop() error
	Force() error
}

// ProxyConfig specifies the configuration for the http proxy
type ProxyConfig struct {
	// Port on which the proxy will be running
	ListeningPort uint
}

// Target defines the upstream target  to forward requests to
type Target struct {
	// Port to redirect traffic to
	Port uint
}

// Proxy defines the parameters used by the proxy for processing http requests and its execution state
type proxy struct {
	config     ProxyConfig
	target     Target
	disruption Disruption
	srv        *http.Server
}

// NewProxy return a new HttpProxy
func NewProxy(target Target, disruption Disruption, config ProxyConfig) (Proxy, error) {
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

// httpClient defines the method for executing HTTP requests. It is used to allow mocking
// the client in tests
type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// httpHandler implements a http.Handler for disrupting request to a upstream server
type httpHandler struct {
	upstreamURL url.URL
	disruption  Disruption
	client      httpClient
}

func (h *httpHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var statusCode int
	body := io.NopCloser(strings.NewReader(h.disruption.ErrorBody))

	excluded := contains(h.disruption.Excluded, req.URL.Path)

	if !excluded && h.disruption.ErrorRate > 0 && rand.Float32() <= h.disruption.ErrorRate {
		// force error code
		statusCode = int(h.disruption.ErrorCode)
	} else {
		req.Host = h.upstreamURL.Host
		req.URL.Host = h.upstreamURL.Host
		req.URL.Scheme = h.upstreamURL.Scheme
		req.RequestURI = ""
		originServerResponse, srvErr := h.client.Do(req)
		if srvErr != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(rw, srvErr)
			return
		}

		statusCode = originServerResponse.StatusCode
		body = originServerResponse.Body

		defer func() {
			_ = originServerResponse.Body.Close()
		}()
	}

	if !excluded && h.disruption.AverageDelay > 0 {
		delay := int(h.disruption.AverageDelay)
		if h.disruption.DelayVariation > 0 {
			delay = delay + int(h.disruption.DelayVariation) - 2*rand.Intn(int(h.disruption.DelayVariation))
		}
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	// return response to the client
	// TODO: return headers
	rw.WriteHeader(statusCode)

	// ignore errors writing body, nothing to do.
	_, _ = io.Copy(rw, body)
}

// Start starts the execution of the proxy
func (p *proxy) Start() error {
	upstreamURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", p.target.Port))
	if err != nil {
		return err
	}

	handler := &httpHandler{
		upstreamURL: *upstreamURL,
		disruption:  p.disruption,
		client:      http.DefaultClient,
	}

	p.srv = &http.Server{
		Addr:    fmt.Sprintf(":%d", p.config.ListeningPort),
		Handler: handler,
	}

	err = p.srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
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
