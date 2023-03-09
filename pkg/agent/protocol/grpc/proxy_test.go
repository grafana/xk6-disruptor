package grpc

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol/grpc/test"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func Test_Validations(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		disruption  Disruption
		config      ProxyConfig
		expectError bool
	}{
		{
			title: "valid defaults",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				StatusCode:     0,
				StatusMessage:  "",
			},
			config: ProxyConfig{
				ListeningPort: 8080,
				Port:          80,
			},
			expectError: false,
		},
		{
			title: "invalid listening port",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				StatusCode:     0,
				StatusMessage:  "",
			},
			config: ProxyConfig{
				ListeningPort: 0,
				Port:          80,
			},
			expectError: true,
		},
		{
			title: "invalid target port",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				StatusCode:     0,
				StatusMessage:  "",
			},
			config: ProxyConfig{
				ListeningPort: 8080,
				Port:          0,
			},
			expectError: true,
		},
		{
			title: "target port equals listening port",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				StatusCode:     0,
				StatusMessage:  "",
			},
			config: ProxyConfig{
				ListeningPort: 80,
				Port:          80,
			},
			expectError: true,
		},
		{
			title: "variation larger than average delay",
			disruption: Disruption{
				AverageDelay:   100,
				DelayVariation: 200,
				ErrorRate:      0.0,
				StatusCode:     0,
				StatusMessage:  "",
			},
			config: ProxyConfig{
				ListeningPort: 8080,
				Port:          80,
			},
			expectError: true,
		},
		{
			title: "valid error rate",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.1,
				StatusCode:     int32(codes.Internal),
				StatusMessage:  "",
			},
			config: ProxyConfig{
				ListeningPort: 8080,
				Port:          80,
			},
			expectError: false,
		},
		{
			title: "valid delay and variation",
			disruption: Disruption{
				AverageDelay:   100,
				DelayVariation: 10,
				ErrorRate:      0.0,
				StatusCode:     0,
				StatusMessage:  "",
			},
			config: ProxyConfig{
				ListeningPort: 8080,
				Port:          80,
			},
			expectError: false,
		},
		{
			title: "invalid error code",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				StatusCode:     0,
				StatusMessage:  "",
			},
			config: ProxyConfig{
				ListeningPort: 8080,
				Port:          80,
			},
			expectError: true,
		},
		{
			title: "negative error rate",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      -1.0,
				StatusCode:     0,
				StatusMessage:  "",
			},
			config: ProxyConfig{
				ListeningPort: 8080,
				Port:          80,
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			_, err := NewProxy(
				tc.config,
				tc.disruption,
			)
			if !tc.expectError && err != nil {
				t.Errorf("failed: %v", err)
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
			}
		})
	}
}

func Test_ProxyHandler(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		title        string
		disruption   Disruption
		config       ProxyConfig
		request      *test.PingRequest
		response     *test.PingResponse
		expectStatus codes.Code
	}

	testCases := []TestCase{
		{
			title: "default proxy",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				StatusCode:     0,
				StatusMessage:  "",
			},
			config: ProxyConfig{
				Port:          8080,
				ListeningPort: 9080,
			},
			request: &test.PingRequest{
				Error:   0,
				Message: "ping",
			},
			response: &test.PingResponse{
				Message: "ping",
			},
			expectStatus: codes.OK,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			// start test server
			l, err := net.Listen("tcp", fmt.Sprintf(":%d", tc.config.Port))
			if err != nil {
				t.Errorf("error starting test server in port %d: %v", tc.config.Port, err)
				return
			}
			srv := grpc.NewServer()
			test.RegisterPingServiceServer(srv, test.NewPingServer())
			go func() {
				if serr := srv.Serve(l); err != nil {
					t.Logf("error in the server: %v", serr)
				}
			}()

			// start proxy
			proxy, err := NewProxy(tc.config, tc.disruption)
			if err != nil {
				t.Errorf("error starting proxy: %v", err)
				return
			}
			defer func() {
				_ = proxy.Stop()
			}()

			// TODO: check for proxy start error
			go func() {
				perr := proxy.Start()
				t.Errorf("error starting proxy: %v", perr)
			}()

			// connect client to proxy
			conn, err := grpc.DialContext(
				context.TODO(),
				fmt.Sprintf(":%d", tc.config.ListeningPort),
				grpc.WithInsecure(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				_ = conn.Close()
			}()

			client := test.NewPingServiceClient(conn)

			var headers metadata.MD
			response, err := client.Ping(context.TODO(), tc.request, grpc.Header(&headers))
			if err != nil && tc.expectStatus == codes.OK {
				t.Errorf("unexpected error %v", err)
				return
			}

			// got an error but it is not due to the grpc status
			s, ok := status.FromError(err)
			if !ok {
				t.Errorf("unexpected error %v", err)
				return
			}

			if s.Code() != tc.expectStatus {
				t.Errorf("expected '%s' but got '%s'", tc.expectStatus.String(), s.Code().String())
				return
			}

			if !test.CompareResponses(response, tc.response) {
				t.Errorf("expected '%s' but got '%s'", tc.response, response)
				return
			}
		})
	}
}
