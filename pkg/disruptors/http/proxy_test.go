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
		proxy      Proxy
		handler    func(http.ResponseWriter, *http.Request)
		path       string
		statusCode int
	}

	testCases := []TestCase{
		{
			title: "default proxy",
			proxy: Proxy{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			handler:    return200Handler,
			path:       "",
			statusCode: 200,
		},
		{
			title: "Error code 500",
			proxy: Proxy{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				ErrorCode:      500,
				Excluded:       nil,
			},
			handler:    return200Handler,
			path:       "",
			statusCode: 500,
		},
		{
			title: "Exclude path",
			proxy: Proxy{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				ErrorCode:      500,
				Excluded:       []string{"/excluded/path"},
			},
			handler:    return200Handler,
			path:       "/excluded/path",
			statusCode: 200,
		},
		{
			title: "Not Excluded path",
			proxy: Proxy{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				ErrorCode:      500,
				Excluded:       []string{"/excluded/path"},
			},
			handler:    return200Handler,
			path:       "/non-excluded/path",
			statusCode: 500,
		},
	}

	proxyPort := 9080 // proxy ports will be in the range 9080-9089
	srvPort := 9090   // server ports will be in the range 8090-9099
	for i, tc := range testCases {
		// ensure unique ports for each test but limit port range
		tc.proxy.Port = uint(proxyPort + (i % 10))
		tc.proxy.Target = uint(srvPort + (i % 10))

		t.Run(tc.title, func(t *testing.T) {
			// create and start upstream server
			srv := &http.Server{
				Addr:    fmt.Sprintf("127.0.0.1:%d", tc.proxy.Target),
				Handler: http.HandlerFunc(tc.handler),
			}
			go func() {
				srv.ListenAndServe()
			}()

			// start proxy
			go func() {
				tc.proxy.Start()
			}()

			// ensure upstream server and proxy are stopped
			defer func() {
				srv.Close()
				tc.proxy.Force()
			}()

			// wait for proxy and upstream server to start
			time.Sleep(1 * time.Second)

			proxyURL := fmt.Sprintf("http://127.0.0.1:%d%s", tc.proxy.Port, tc.path)

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
