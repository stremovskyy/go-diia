// Command diia-callback-relay forwards sandbox Diia callbacks to Webhook.site.
package main

import (
	"context"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	callbackPath = "/diia/documents-receiver"
	maxWait      = 25 * time.Second
)

var webhookTokenPattern = regexp.MustCompile(`^/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type relay struct {
	target *url.URL
	client *http.Client
}

func main() {
	target, err := parseWebhookURL(os.Getenv("WEBHOOK_URL"))
	if err != nil {
		log.Fatal("WEBHOOK_URL must be an HTTPS webhook.site token URL")
	}

	server := &http.Server{
		Addr:              ":8080",
		Handler:           routes(&relay{target: target, client: &http.Client{Timeout: maxWait, CheckRedirect: noRedirect}}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       time.Minute,
	}

	log.Print("Diia sandbox callback relay listening on :8080")
	log.Fatal(server.ListenAndServe())
}

func noRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

func parseWebhookURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u == nil || u.Scheme != "https" || u.Hostname() != "webhook.site" ||
		(u.Port() != "" && u.Port() != "443") || u.User != nil || u.Fragment != "" ||
		u.RawQuery != "" || !webhookTokenPattern.MatchString(u.Path) {
		return nil, errors.New("invalid webhook URL")
	}

	return u, nil
}

func routes(r *relay) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST "+callbackPath, r.forward)
	return mux
}

func (r *relay) forward(w http.ResponseWriter, request *http.Request) {
	if strings.TrimSpace(request.Header.Get("X-Document-Request-Trace-Id")) == "" {
		writeResult(w, http.StatusBadRequest, false)
		return
	}

	mediaType, parameters, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" || parameters["boundary"] == "" {
		writeResult(w, http.StatusBadRequest, false)
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), maxWait)
	defer cancel()

	target := *r.target
	target.RawQuery = request.URL.RawQuery
	outbound, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), request.Body)
	if err != nil {
		writeResult(w, http.StatusBadGateway, false)
		return
	}
	outbound.ContentLength = request.ContentLength
	copyEndToEndHeaders(outbound.Header, request.Header)

	response, err := r.client.Do(outbound)
	if err != nil {
		log.Printf("Diia callback relay upstream error: %T", err)
		writeResult(w, http.StatusBadGateway, false)
		return
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		log.Printf("Diia callback relay upstream status: %d", response.StatusCode)
		writeResult(w, http.StatusBadGateway, false)
		return
	}

	writeResult(w, http.StatusOK, true)
}

func copyEndToEndHeaders(destination, source http.Header) {
	hopByHop := map[string]bool{
		"Connection":          true,
		"Proxy-Connection":    true,
		"Keep-Alive":          true,
		"Proxy-Authenticate":  true,
		"Proxy-Authorization": true,
		"Te":                  true,
		"Trailer":             true,
		"Transfer-Encoding":   true,
		"Upgrade":             true,
	}
	for _, connection := range source.Values("Connection") {
		for _, token := range strings.Split(connection, ",") {
			hopByHop[http.CanonicalHeaderKey(strings.TrimSpace(token))] = true
		}
	}

	for name, values := range source {
		if hopByHop[http.CanonicalHeaderKey(name)] {
			continue
		}
		for _, value := range values {
			destination.Add(name, value)
		}
	}
}

func writeResult(w http.ResponseWriter, status int, success bool) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if success {
		_, _ = io.WriteString(w, `{"success":true}`)
		return
	}
	_, _ = io.WriteString(w, `{"success":false,"error":"callback relay failed"}`)
}
