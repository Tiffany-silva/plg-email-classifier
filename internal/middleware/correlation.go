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

package middleware

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/http"
	"os"
)

const correlationIDHeader = "X-CSM-Correlation-ID"

type correlationIDKey struct{}

// CorrelationID reads the X-CSM-Correlation-ID request header or generates a
// UUID v4 if absent. The ID is stored in the context for automatic inclusion in
// slog records and echoed in the response header so callers can reference it.
func CorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(correlationIDHeader)
		if id == "" {
			id = newCorrelationID()
		}
		w.Header().Set(correlationIDHeader, id)
		ctx := context.WithValue(r.Context(), correlationIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CorrelationIDFromContext returns the correlation ID stored in ctx, or "" if
// the CorrelationID middleware was not applied.
func CorrelationIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(correlationIDKey{}).(string)
	return v
}

func newCorrelationID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("correlationid: failed to read random bytes: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// ctxHandler wraps a slog.Handler to inject the correlation ID and, when
// present, the authenticated caller ID from the request context into every log
// record produced via *Context methods.
type ctxHandler struct {
	inner slog.Handler
}

func (h ctxHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h ctxHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := CorrelationIDFromContext(ctx); id != "" {
		r.AddAttrs(slog.String("correlationID", id))
	}
	if user := UserInfoFromContext(ctx); user != nil {
		r.AddAttrs(slog.String("userID", user.UserID))
	}
	return h.inner.Handle(ctx, r)
}

func (h ctxHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return ctxHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h ctxHandler) WithGroup(name string) slog.Handler {
	return ctxHandler{inner: h.inner.WithGroup(name)}
}

// ConfigureLogger sets the default slog logger to a handler that adds the
// correlation ID (and caller ID) from the request context to every record.
// Call once at startup before any log statements.
func ConfigureLogger() {
	slog.SetDefault(slog.New(ctxHandler{inner: slog.NewJSONHandler(os.Stderr, nil)}))
}
