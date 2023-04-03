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
)

// fakeHTTPClient mocks the execution of a request returning the predefines
// status and body
type fakeHTTPClient struct {
	status int
	body   []byte
}

func (f *fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	resp := &http.Response{
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		StatusCode:    f.status,
		Status:        http.StatusText(f.status),
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
		title          string
		disruption     Disruption
		config         ProxyConfig
		method         string
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
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			client := &fakeHTTPClient{
				body:   tc.body,
				status: tc.expectedStatus,
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

			var body bytes.Buffer
			_, _ = io.Copy(&body, resp.Body)
			if !bytes.Equal(tc.expectedBody, body.Bytes()) {
				t.Errorf("expected body '%s' but '%s' received ", tc.expectedBody, body.Bytes())
				return
			}
		})
	}
}
