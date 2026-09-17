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
	"encoding/json"
	"net/http"
)

// UserInfo holds the authenticated caller's identity. In this standalone
// service identity comes from a shared API key rather than a JWT; when deployed
// behind a gateway that already validates a JWT, swap Auth for a JWT variant and
// keep this same context contract so handlers do not change.
type UserInfo struct {
	UserID string
}

type userInfoKey struct{}

// UserInfoFromContext returns the authenticated caller, or nil if Auth was not
// applied (or rejected the request).
func UserInfoFromContext(ctx context.Context) *UserInfo {
	v, _ := ctx.Value(userInfoKey{}).(*UserInfo)
	return v
}

// WithUserInfo returns a copy of ctx carrying user. Used by tests to inject a
// caller without going through the real Auth check.
func WithUserInfo(ctx context.Context, user *UserInfo) context.Context {
	return context.WithValue(ctx, userInfoKey{}, user)
}

// Auth validates the X-API-Key header against an allow-list. When the allow-list
// is empty (local development) authentication is disabled and every request is
// treated as a fixed local caller. Keys are never logged.
func Auth(allowed map[string]struct{}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(allowed) == 0 {
				ctx := WithUserInfo(r.Context(), &UserInfo{UserID: "local-dev"})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			key := r.Header.Get("X-API-Key")
			if _, ok := allowed[key]; key == "" || !ok {
				writeUnauthorized(w)
				return
			}
			ctx := WithUserInfo(r.Context(), &UserInfo{UserID: "api-caller"})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "You are not authorized to perform this action. Please try again.",
	})
}
