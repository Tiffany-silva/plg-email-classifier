// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/wso2-open-operations/plg-email-classifier/internal/anthropic"
	"github.com/wso2-open-operations/plg-email-classifier/internal/emailclassifier"
	"github.com/wso2-open-operations/plg-email-classifier/internal/handler"
	"github.com/wso2-open-operations/plg-email-classifier/internal/middleware"
)

func main() {
	loadDotEnv(".env")
	middleware.ConfigureLogger()

	// Build the classifier service (this is the whole point of the process, so
	// it is always constructed; the model path can be turned off for keyless
	// testing via LLM_ENABLED=false).
	classifierSvc, refresher := buildClassifier()
	emailHandler := handler.NewEmailClassificationHandler(classifierSvc)
	healthHandler := handler.NewHealthHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler.Health)
	mux.HandleFunc("POST /classify-email", emailHandler.ClassifyEmail)

	// Middleware chain (outermost first), mirroring the CSM portal:
	// SecurityHeaders -> CORS -> CorrelationID -> Auth -> Logger -> Mux.
	callerKeys := splitComma(os.Getenv("CALLER_API_KEYS"))
	allowed := make(map[string]struct{}, len(callerKeys))
	for _, k := range callerKeys {
		allowed[k] = struct{}{}
	}
	var h http.Handler = mux
	h = middleware.Logger(h)
	h = middleware.Auth(allowed)(h)
	h = middleware.CorrelationID(h)
	h = middleware.CORS(splitComma(os.Getenv("CORS_ALLOWED_ORIGINS")))(h)
	h = middleware.SecurityHeaders(h)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if refresher != nil {
		go refresher.Start(ctx)
	}

	addr := ":" + mustPort("PORT", "8080")
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		slog.Error("failed to bind", "addr", addr, "err", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("server exited", "err", err)
			os.Exit(1)
		}
	}()
	slog.Info("Email Classification service started", "addr", addr)

	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	slog.Info("Email Classification service stopped")
}

// buildClassifier wires the classification service and the background
// disposable-list refresher from environment configuration.
//
//	LLM_ENABLED                 default true; false runs the deterministic
//	                            cascade only (no ANTHROPIC_API_KEY required) -
//	                            unknown reachable domains resolve to "unknown".
//	ANTHROPIC_API_KEY           required when the model path is on.
//	CLASSIFIER_MODEL            default claude-haiku-4-5-20251001.
//	CACHE_TTL                   domain-verdict cache TTL (default 48h).
//	LLM_TIMEOUT                 per-call model timeout (default 8s).
//	MX_TIMEOUT                  per-call DNS/MX timeout (default 3s).
//	DISPOSABLE_LIST_PATH        file loaded at startup and written by refresher.
//	FREE_PROVIDER_LIST_PATH     optional free-provider domain file.
//	DISPOSABLE_LIST_URL         refresher source (default public registry).
//	DISPOSABLE_REFRESH_INTERVAL poll interval (default 2h; 0 disables polling).
func buildClassifier() (*emailclassifier.Service, *emailclassifier.Refresher) {
	cacheTTL := parseDurationEnv("CACHE_TTL", 48*time.Hour)
	llmTimeout := parseDurationEnv("LLM_TIMEOUT", 8*time.Second)
	mxTimeout := parseDurationEnv("MX_TIMEOUT", 3*time.Second)

	disposablePath := strings.TrimSpace(os.Getenv("DISPOSABLE_LIST_PATH"))
	if disposablePath != "" {
		if n, err := emailclassifier.LoadDisposable(disposablePath); err != nil {
			slog.Warn("could not load disposable list (relying on refresher/seed)", "path", disposablePath, "err", err)
		} else {
			slog.Info("loaded disposable domains", "count", n)
		}
	}
	if freePath := strings.TrimSpace(os.Getenv("FREE_PROVIDER_LIST_PATH")); freePath != "" {
		if n, err := emailclassifier.LoadFreeProviders(freePath); err != nil {
			slog.Error("invalid FREE_PROVIDER_LIST_PATH", "path", freePath, "err", err)
			os.Exit(1)
		} else {
			slog.Info("loaded free providers", "count", n)
		}
	}

	llmEnabled := getbool("LLM_ENABLED", true)
	var llm emailclassifier.LLMClassifier
	if llmEnabled {
		client := anthropic.NewClient(anthropic.Config{
			APIKey:  mustEnv("ANTHROPIC_API_KEY"),
			Model:   envOrDefault("CLASSIFIER_MODEL", "claude-haiku-4-5-20251001"),
			Timeout: llmTimeout,
		})
		llm = emailclassifier.NewAnthropicLLM(client)
	} else {
		slog.Info("model path disabled; unknown reachable domains resolve to 'unknown'")
	}

	svc := emailclassifier.NewService(emailclassifier.NewMemoryCache(cacheTTL), llm, mxTimeout)

	var refresher *emailclassifier.Refresher
	interval := parseDurationEnv("DISPOSABLE_REFRESH_INTERVAL", 2*time.Hour)
	url := envOrDefault("DISPOSABLE_LIST_URL", "https://dmails.doodadlabs.org/data/domains.txt")
	if interval > 0 && url != "" {
		refresher = emailclassifier.NewRefresher(url, disposablePath, interval)
		slog.Info("disposable-list auto-refresh enabled", "url", url, "interval", interval.String())
	}
	slog.Info("classifier ready", "llm", llmEnabled)
	return svc, refresher
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable is not set", "key", key)
		os.Exit(1)
	}
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getbool(key string, def bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		slog.Warn("invalid boolean environment variable; using default", "key", key, "value", raw, "default", def)
		return def
	}
	return b
}

func parseDurationEnv(key string, def time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		slog.Warn("invalid duration environment variable; using default", "key", key, "value", raw, "default", def.String())
		return def
	}
	return d
}

// mustPort returns a bare port number (e.g. "8080"), exiting if invalid.
func mustPort(key, def string) string {
	v := envOrDefault(key, def)
	port, err := strconv.Atoi(v)
	if err != nil || port < 1 || port > 65535 {
		slog.Error("environment variable must be a plain port number (e.g. \"8080\")", "key", key, "value", v)
		os.Exit(1)
	}
	return v
}

// loadDotEnv reads a .env file and sets any unset environment variables from it.
// Silently ignored if the file does not exist.
func loadDotEnv(path string) {
	f, err := os.Open(path) // #nosec G304 -- always the hardcoded literal ".env"
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("loadDotEnv: failed to open .env file", "err", err)
		}
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("loadDotEnv: error reading .env file", "err", err)
	}
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}
