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
