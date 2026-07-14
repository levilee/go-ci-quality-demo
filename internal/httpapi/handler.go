package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

type API struct {
	upstreamURL string
	client      *http.Client
}

func NewHandler(upstreamURL string, client *http.Client) http.Handler {
	api := &API{upstreamURL: upstreamURL, client: client}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", getOnly(api.health))
	mux.HandleFunc("/api/greet", getOnly(api.greet))
	mux.HandleFunc("/api/calculate", getOnly(api.calculate))
	mux.HandleFunc("/api/upstream", getOnly(api.callUpstream))
	return mux
}

func getOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		next(w, r)
	}
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) greet(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "world"
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "hello, " + name})
}

func (a *API) calculate(w http.ResponseWriter, r *http.Request) {
	left, err := requiredInteger(r, "a")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	right, err := requiredInteger(r, "b")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	operation := r.URL.Query().Get("operation")
	var result int64
	switch operation {
	case "add":
		result = left + right
	case "subtract":
		result = left - right
	case "multiply":
		result = left * right
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "operation must be one of: add, subtract, multiply",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"a":         left,
		"b":         right,
		"operation": operation,
		"result":    result,
	})
}

func requiredInteger(r *http.Request, name string) (int64, error) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return 0, fmt.Errorf("%s is required", name)
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer", name)
	}
	return parsed, nil
}

func (a *API) callUpstream(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, a.upstreamURL, nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "invalid upstream configuration"})
		return
	}
	req.Header.Set("Accept", "text/plain")

	resp, err := a.client.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream request failed"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream response could not be read"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"upstreamStatus": resp.StatusCode,
		"body":           string(body),
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		panic(fmt.Sprintf("encode response: %v", err))
	}
}
