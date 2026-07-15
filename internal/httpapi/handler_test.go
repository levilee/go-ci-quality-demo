package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	NewHandler("http://unused", http.DefaultClient).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	assertJSONField(t, recorder, "status", "ok")
}

func TestGreet(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "default", url: "/api/greet", want: "hello, world"},
		{name: "provided name", url: "/api/greet?name=Codex", want: "hello, Codex"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, test.url, nil)
			NewHandler("http://unused", http.DefaultClient).ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			assertJSONField(t, recorder, "message", test.want)
		})
	}
}

func TestCalculate(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantCode  int
		wantValue int64
		wantError string
	}{
		{name: "add", url: "/api/calculate?a=7&b=5&operation=add", wantCode: http.StatusOK, wantValue: 12},
		{name: "subtract", url: "/api/calculate?a=7&b=5&operation=subtract", wantCode: http.StatusOK, wantValue: 2},
		{name: "multiply", url: "/api/calculate?a=-3&b=5&operation=multiply", wantCode: http.StatusOK, wantValue: -15},
		{name: "missing a", url: "/api/calculate?b=5&operation=add", wantCode: http.StatusBadRequest, wantError: "a is required"},
		{name: "invalid b", url: "/api/calculate?a=7&b=text&operation=add", wantCode: http.StatusBadRequest, wantError: "b must be a valid integer"},
		{name: "invalid operation", url: "/api/calculate?a=7&b=5&operation=divide", wantCode: http.StatusBadRequest, wantError: "operation must be one of: add, subtract, multiply"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, test.url, nil)
			NewHandler("http://unused", http.DefaultClient).ServeHTTP(recorder, request)

			if recorder.Code != test.wantCode {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantCode)
			}

			var response struct {
				Result int64  `json:"result"`
				Error  string `json:"error"`
			}
			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Result != test.wantValue {
				t.Fatalf("result = %d, want %d", response.Result, test.wantValue)
			}
			if response.Error != test.wantError {
				t.Fatalf("error = %q, want %q", response.Error, test.wantError)
			}
		})
	}
}

func TestCalculateRejectsPost(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/calculate?a=1&b=2&operation=add", nil)
	NewHandler("http://unused", http.DefaultClient).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
}

func TestPOCQualityGateBlocksInvalidExpectation(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/calculate?a=7&b=5&operation=add", nil)
	NewHandler("http://unused", http.DefaultClient).ServeHTTP(recorder, request)

	var response struct {
		Result int64 `json:"result"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	const deliberatelyWrongExpectedResult int64 = 999
	if response.Result != deliberatelyWrongExpectedResult {
		t.Fatalf("POC intentional failure: result = %d, want %d", response.Result, deliberatelyWrongExpectedResult)
	}
}

func TestCallUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("upstream-ok"))
	}))
	defer upstream.Close()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/upstream", nil)
	NewHandler(upstream.URL, upstream.Client()).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response struct {
		UpstreamStatus int    `json:"upstreamStatus"`
		Body           string `json:"body"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.UpstreamStatus != http.StatusAccepted || response.Body != "upstream-ok" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestCallUpstreamFailure(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network unavailable")
	})}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/upstream", nil)
	NewHandler("http://upstream.invalid", client).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadGateway)
	}
}

func assertJSONField(t *testing.T, recorder *httptest.ResponseRecorder, field, want string) {
	t.Helper()
	var response map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response[field] != want {
		t.Fatalf("%s = %q, want %q", field, response[field], want)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
