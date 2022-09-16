package http

import (
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
		target     HttpProxyTarget
		disruption HttpDisruption
		config     HttpProxyConfig
		handler    func(http.ResponseWriter, *http.Request)
		path       string
		statusCode int
	}

	testCases := []TestCase{
		{
			title: "default proxy",
			disruption: HttpDisruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			target: HttpProxyTarget{
				Port: 0, // to be set in the test
			},
			config: HttpProxyConfig{
				ListeningPort: 0, // to be set in the test
			},
			handler:    return200Handler,
			path:       "",
			statusCode: 200,
		},
		{
			title: "Error code 500",
			disruption: HttpDisruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				ErrorCode:      500,
				Excluded:       nil,
			},
			target: HttpProxyTarget{
				Port: 0, // to be set in the test
			},
			config: HttpProxyConfig{
				ListeningPort: 0, // to be set in the test
			},
			handler:    return200Handler,
			path:       "",
			statusCode: 500,
		},
		{
			title: "Exclude path",
			disruption: HttpDisruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				ErrorCode:      500,
				Excluded:       []string{"/excluded/path"},
			},
			target: HttpProxyTarget{
				Port: 0, // to be set in the test
			},
			config: HttpProxyConfig{
				ListeningPort: 0, // to be set in the test
			},
			handler:    return200Handler,
			path:       "/excluded/path",
			statusCode: 200,
		},
		{
			title: "Not Excluded path",
			disruption: HttpDisruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				ErrorCode:      500,
				Excluded:       []string{"/excluded/path"},
			},
			target: HttpProxyTarget{
				Port: 0, // to be set in the test
			},
			config: HttpProxyConfig{
				ListeningPort: 0, // to be set in the test
			},
			handler:    return200Handler,
			path:       "/non-excluded/path",
			statusCode: 500,
		},
	}

	proxyPort := uint(32080) // proxy ports will be in the range 32080-32089
	srvPort := uint(32090)   // server ports will be in the range 32090-32099
	for i, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			// ensure unique ports for each test but limit port range
			tc.config.ListeningPort = proxyPort + uint(i%10)
			tc.target.Port = srvPort + uint(i%10)

			// create the proxy
			proxy, err := NewHttpProxy(
				tc.target,
				tc.disruption,
				tc.config,
			)
			if err != nil {
				t.Errorf("error creating proxy: %v", err)
				return
			}

			// create and start upstream server
			srv := &http.Server{
				Addr:    fmt.Sprintf("127.0.0.1:%d", tc.target.Port),
				Handler: http.HandlerFunc(tc.handler),
			}
			go func() {
				srv.ListenAndServe()
			}()

			// start proxy
			go func() {
				proxy.Start()
			}()

			// ensure upstream server and proxy are stopped
			defer func() {
				srv.Close()
				proxy.Force()
			}()

			// wait for proxy and upstream server to start
			time.Sleep(1 * time.Second)

			proxyURL := fmt.Sprintf("http://127.0.0.1:%d%s", tc.config.ListeningPort, tc.path)

			resp, err := http.Get(proxyURL)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tc.statusCode != resp.StatusCode {
				t.Errorf("expected status code '%d' but '%d' received ", tc.statusCode, resp.StatusCode)
				return
			}
		})
	}
}
