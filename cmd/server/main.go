package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"go-ci-quality-demo/internal/httpapi"
)

func main() {
	port := envOrDefault("PORT", "8080")
	upstreamURL := envOrDefault("UPSTREAM_URL", "https://api.github.com/zen")

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.NewHandler(upstreamURL, &http.Client{Timeout: 5 * time.Second}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
