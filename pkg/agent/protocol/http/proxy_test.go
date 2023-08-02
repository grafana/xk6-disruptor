package http

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
)

// fakeHTTPClient mocks the execution of a request returning the predefines
// status and body
type fakeHTTPClient struct {
	status  int
	headers http.Header
	body    []byte
}

func (f *fakeHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	resp := &http.Response{
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		StatusCode:    f.status,
		Status:        http.StatusText(f.status),
		Header:        f.headers,
		Body:          io.NopCloser(strings.NewReader(string(f.body))),
		ContentLength: int64(len(f.body)),
	}

	return resp, nil
}

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
				ErrorCode:      0,
				Excluded:       nil,
			},
			config: ProxyConfig{
				ListenAddress:   ":8080",
				UpstreamAddress: "http://127.0.0.1:80",
			},
			expectError: false,
		},
		{
			title: "invalid listening address",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			config: ProxyConfig{
				ListenAddress:   "",
				UpstreamAddress: "http://127.0.0.1:80",
			},
			expectError: true,
		},
		{
			title: "invalid upstream address",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			config: ProxyConfig{
				ListenAddress:   ":8080",
				UpstreamAddress: "",
			},
			expectError: true,
		},
		{
			title: "variation larger than average delay",
			disruption: Disruption{
				AverageDelay:   100,
				DelayVariation: 200,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			config: ProxyConfig{
				ListenAddress:   ":8080",
				UpstreamAddress: "http://127.0.0.1:80",
			},
			expectError: true,
		},
		{
			title: "valid error rate",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.1,
				ErrorCode:      500,
				Excluded:       nil,
			},
			config: ProxyConfig{
				ListenAddress:   ":8080",
				UpstreamAddress: "http://127.0.0.1:80",
			},
			expectError: false,
		},
		{
			title: "valid delay and variation",
			disruption: Disruption{
				AverageDelay:   100,
				DelayVariation: 10,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			config: ProxyConfig{
				ListenAddress:   ":8080",
				UpstreamAddress: "http://127.0.0.1:80",
			},
			expectError: false,
		},
		{
			title: "invalid error code",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			config: ProxyConfig{
				ListenAddress:   ":8080",
				UpstreamAddress: "http://127.0.0.1:80",
			},
			expectError: true,
		},
		{
			title: "negative error rate",
			disruption: Disruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      -1.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			config: ProxyConfig{
				ListenAddress:   ":8080",
				UpstreamAddress: "http://127.0.0.1:80",
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
		title           string
		disruption      Disruption
		config          ProxyConfig
		method          string
		path            string
		statusCode      int
		headers         http.Header
		body            []byte
		expectedStatus  int
		expectedHeaders http.Header
		expectedBody    []byte
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
			config: ProxyConfig{
				ListenAddress:   ":9080",
				UpstreamAddress: "http://127.0.0.1:8080",
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
			config: ProxyConfig{
				ListenAddress:   ":9080",
				UpstreamAddress: "http://127.0.0.1:8080",
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
			config: ProxyConfig{
				ListenAddress:   ":9080",
				UpstreamAddress: "http://127.0.0.1:8080",
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
			config: ProxyConfig{
				ListenAddress:   ":9080",
				UpstreamAddress: "http://127.0.0.1:8080",
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
			config: ProxyConfig{
				ListenAddress:   ":9080",
				UpstreamAddress: "http://127.0.0.1:8080",
			},
			path:           "",
			statusCode:     200,
			body:           []byte("content body"),
			expectedStatus: 500,
			expectedBody:   []byte("{\"error\": 500, \"message\":\"internal server error\"}"),
		},
		{
			title: "Headers are preserved when endpoint is skipped",
			disruption: Disruption{
				Excluded: []string{"/excluded"},
			},
			config: ProxyConfig{
				ListenAddress:   ":9080",
				UpstreamAddress: "http://127.0.0.1:8080",
			},
			path:       "/excluded",
			statusCode: 200,
			headers: http.Header{
				"X-Test-Header": []string{"A-Test"},
			},
			body:           []byte("content body"),
			expectedStatus: 200,
			expectedHeaders: http.Header{
				"X-Test-Header": []string{"A-Test"},
			},
			expectedBody: []byte("content body"),
		},
		{
			title: "Headers are preserved when errors are not injected",
			disruption: Disruption{
				ErrorRate: 0,
			},
			config: ProxyConfig{
				ListenAddress:   ":9080",
				UpstreamAddress: "http://127.0.0.1:8080",
			},
			statusCode: 200,
			headers: http.Header{
				"X-Test-Header": []string{"A-Test"},
			},
			body:           []byte("content body"),
			expectedStatus: 200,
			expectedHeaders: http.Header{
				"X-Test-Header": []string{"A-Test"},
			},
			expectedBody: []byte("content body"),
		},
		{
			title: "Headers are discarded when errors are injected",
			disruption: Disruption{
				ErrorRate: 1.0,
				ErrorCode: 500,
			},
			config: ProxyConfig{
				ListenAddress:   ":9080",
				UpstreamAddress: "http://127.0.0.1:8080",
			},
			statusCode: 200,
			headers: http.Header{
				"X-Test-Header": []string{"A-Test"},
			},
			body:            []byte("content body"),
			expectedStatus:  500,
			expectedHeaders: http.Header{},
			expectedBody:    nil,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			client := &fakeHTTPClient{
				body:    tc.body,
				status:  tc.expectedStatus,
				headers: tc.headers,
			}

			upstreamURL, err := url.Parse(tc.config.UpstreamAddress)
			if err != nil {
				t.Errorf("error parsing upstream address %v", err)
				return
			}
			handler := &httpHandler{
				upstreamURL: *upstreamURL,
				client:      client,
				disruption:  tc.disruption,
				metrics:     &protocol.MetricMap{},
			}

			reqURL := fmt.Sprintf("http://%s%s", tc.config.ListenAddress, tc.path)
			req := httptest.NewRequest(tc.method, reqURL, strings.NewReader(string(tc.body)))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			resp := recorder.Result()

			if tc.expectedStatus != resp.StatusCode {
				t.Errorf("expected status code '%d' but '%d' received ", tc.expectedStatus, resp.StatusCode)
				return
			}

			// Compare headers only if either expected or returned have items.
			// We have to check for length explicitly as otherwise a nil map would not be equal to an empty map.
			if len(tc.headers) > 0 || len(tc.expectedHeaders) > 0 {
				if diff := cmp.Diff(tc.expectedHeaders, resp.Header); diff != "" {
					t.Errorf("Expected headers did not match returned:\n%s", diff)
				}
			}

			var body bytes.Buffer
			_, _ = io.Copy(&body, resp.Body)
			if !bytes.Equal(tc.expectedBody, body.Bytes()) {
				t.Errorf("expected body '%s' but '%s' received ", tc.expectedBody, body.Bytes())
				return
			}
		})
	}
}

// TODO: This test covers metrics generated by the handler, but not the proxy. The reason for this is that the proxy is
// currently not easily testable, as it coupled with `http.ListenAndServe`.
func Test_Metrics(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name            string
		config          Disruption
		endpoints       []string
		expectedMetrics map[string]uint
	}{
		{
			name:            "no requests",
			expectedMetrics: map[string]uint{},
		},
		{
			name: "requests",
			config: Disruption{
				Excluded:  []string{"/excluded"},
				ErrorRate: 1.0,
				ErrorCode: http.StatusTeapot,
			},
			endpoints: []string{"/included", "/excluded"},
			expectedMetrics: map[string]uint{
				protocol.MetricRequests:         2,
				protocol.MetricRequestsExcluded: 1,
				protocol.MetricRequestsFaulted:  1,
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			upstreamServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				rw.WriteHeader(http.StatusOK)
			}))

			upstreamURL, err := url.Parse(upstreamServer.URL)
			if err != nil {
				t.Fatalf("error parsing httptest url")
			}

			metrics := protocol.MetricMap{}

			handler := &httpHandler{
				upstreamURL: *upstreamURL,
				disruption:  tc.config,
				client:      http.DefaultClient,
				metrics:     &metrics,
			}

			proxyServer := httptest.NewServer(handler)

			for _, endpoint := range tc.endpoints {
				_, err = http.Get(proxyServer.URL + endpoint)
				if err != nil {
					t.Fatalf("requesting %s: %v", endpoint, err)
				}
			}

			if diff := cmp.Diff(tc.expectedMetrics, metrics.Map()); diff != "" {
				t.Fatalf("expected metrics do not match output:\n%s", diff)
			}
		})
	}
}
