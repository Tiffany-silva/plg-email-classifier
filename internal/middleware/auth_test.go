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
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserInfoFromContext(r.Context()) == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func TestAuthDisabledWhenNoKeys(t *testing.T) {
	h := Auth(nil)(okHandler())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/x", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (auth disabled)", w.Code)
	}
}

func TestAuthRejectsBadKey(t *testing.T) {
	allowed := map[string]struct{}{"good-key": {}}
	h := Auth(allowed)(okHandler())

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/x", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing key: status = %d, want 401", w.Code)
	}

	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("X-API-Key", "good-key")
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("valid key: status = %d, want 200", w.Code)
	}
}
