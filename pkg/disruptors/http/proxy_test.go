package http

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

func Test_Proxy(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		title          string
		target         Target
		disruption     Disruption
		config         ProxyConfig
		path           string
		statusCode     int
		body           []byte
		expectedStatus int
		expectedBody   []byte
	}

	testCases := []TestCase{
		{
			title: "default proxy",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			target: Target{
				Port: 8080,
			},
			config: ProxyConfig{
				ListeningPort: 9080,
			},
			path:           "",
			statusCode:     200,
			body:           []byte("content body"),
			expectedStatus: 200,
			expectedBody:   []byte("content body"),
		},
		{
			title: "Error code 500",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				ErrorCode:      500,
				Excluded:       nil,
			},
			target: Target{
				Port: 8081,
			},
			config: ProxyConfig{
				ListeningPort: 9081,
			},
			path:           "",
			statusCode:     200,
			body:           []byte("content body"),
			expectedStatus: 500,
			expectedBody:   []byte(""),
		},
		{
			title: "Exclude path",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				ErrorCode:      500,
				Excluded:       []string{"/excluded/path"},
			},
			target: Target{
				Port: 8082,
			},
			config: ProxyConfig{
				ListeningPort: 9082,
			},
			path:           "/excluded/path",
			statusCode:     200,
			body:           []byte("content body"),
			expectedStatus: 200,
			expectedBody:   []byte("content body"),
		},
		{
			title: "Not Excluded path",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				ErrorCode:      500,
				Excluded:       []string{"/excluded/path"},
			},
			target: Target{
				Port: 8083,
			},
			config: ProxyConfig{
				ListeningPort: 9083,
			},
			path:           "/non-excluded/path",
			statusCode:     200,
			body:           []byte("content body"),
			expectedStatus: 500,
			expectedBody:   []byte(""),
		},
		{
			title: "Error code 500 with body template",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				ErrorCode:      500,
				ErrorBody:      "{\"error\": 500, \"message\":\"internal server error\"}",
				Excluded:       nil,
			},
			target: Target{
				Port: 8084,
			},
			config: ProxyConfig{
				ListeningPort: 9084,
			},
			path:           "",
			statusCode:     200,
			body:           []byte("content body"),
			expectedStatus: 500,
			expectedBody:   []byte("{\"error\": 500, \"message\":\"internal server error\"}"),
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			// create the proxy
			proxy, err := NewProxy(
				tc.target,
				tc.disruption,
				tc.config,
			)
			if err != nil {
				t.Errorf("error creating proxy: %v", err)
				return
			}

			errChannel := make(chan error)

			// create and start upstream server
			srv := &http.Server{
				Addr: fmt.Sprintf("127.0.0.1:%d", tc.target.Port),
				Handler: http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
					rw.WriteHeader(tc.statusCode)
					_, _ = rw.Write(tc.body)
				}),
			}
			go func(c chan error) {
				svrErr := srv.ListenAndServe()
				if !errors.Is(svrErr, http.ErrServerClosed) {
					c <- svrErr
				}
			}(errChannel)

			// start proxy
			go func(c chan error) {
				proxyErr := proxy.Start()
				if proxyErr != nil {
					c <- proxyErr
				}
			}(errChannel)

			// ensure upstream server and proxy are stopped
			defer func() {
				// ignore errors, nothing to do
				_ = srv.Close()
				_ = proxy.Force()
			}()

			// wait for proxy and upstream server to start
			time.Sleep(1 * time.Second)
			select {
			case err = <-errChannel:
				t.Errorf("error setting up test %v", err)
				return
			default:
			}

			proxyURL := fmt.Sprintf("http://127.0.0.1:%d%s", tc.config.ListeningPort, tc.path)

			resp, err := http.Get(proxyURL)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			defer func() {
				_ = resp.Body.Close()
			}()

			if tc.expectedStatus != resp.StatusCode {
				t.Errorf("expected status code '%d' but '%d' received ", tc.expectedStatus, resp.StatusCode)
				return
			}

			body := make([]byte, resp.ContentLength)
			_, err = resp.Body.Read(body)
			if err != nil && !errors.Is(err, io.EOF) {
				t.Errorf("unexpected error reading response body: %v", err)
				return
			}
			if !bytes.Equal(tc.expectedBody, body) {
				t.Errorf("expected body '%s' but '%s' received ", tc.expectedBody, body)
				return
			}
		})
	}
}
