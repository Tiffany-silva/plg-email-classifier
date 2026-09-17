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

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/plg-email-classifier/internal/middleware"
)

var testUser = &middleware.UserInfo{UserID: "test-caller"}

func withUser(r *http.Request) *http.Request {
	return r.WithContext(middleware.WithUserInfo(r.Context(), testUser))
}

func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Errorf("status = %d, want %d; body: %s", w.Code, want, w.Body.String())
	}
}

func assertContentType(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	if ct := w.Header().Get("Content-Type"); ct != want {
		t.Errorf("Content-Type = %q, want %q", ct, want)
	}
}

func decodeJSON[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(w.Body).Decode(&v); err != nil {
		t.Fatalf("decode body: %v; raw: %s", err, w.Body.String())
	}
	return v
}
