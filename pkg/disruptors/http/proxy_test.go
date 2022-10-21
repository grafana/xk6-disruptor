package http

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func return200Handler(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(200)
}

func Test_Proxy(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		title      string
		target     Target
		disruption Disruption
		config     ProxyConfig
		handler    func(http.ResponseWriter, *http.Request)
		path       string
		statusCode int
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
			handler:    return200Handler,
			path:       "",
			statusCode: 200,
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
			handler:    return200Handler,
			path:       "",
			statusCode: 500,
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
			handler:    return200Handler,
			path:       "/excluded/path",
			statusCode: 200,
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
			handler:    return200Handler,
			path:       "/non-excluded/path",
			statusCode: 500,
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
				Addr:    fmt.Sprintf("127.0.0.1:%d", tc.target.Port),
				Handler: http.HandlerFunc(tc.handler),
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

			if tc.statusCode != resp.StatusCode {
				t.Errorf("expected status code '%d' but '%d' received ", tc.statusCode, resp.StatusCode)
				return
			}
		})
	}
}
