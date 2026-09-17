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
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/plg-email-classifier/internal/emailclassifier"
)

type mockEmailClassifier struct {
	fn func(ctx context.Context, email string) emailclassifier.Result
}

func (m *mockEmailClassifier) Classify(ctx context.Context, email string) emailclassifier.Result {
	if m.fn != nil {
		return m.fn(ctx, email)
	}
	return emailclassifier.Result{}
}

func TestClassifyEmail_Unauthorized(t *testing.T) {
	h := NewEmailClassificationHandler(&mockEmailClassifier{})
	req := httptest.NewRequest(http.MethodPost, "/classify-email", strings.NewReader(`{"email":"x@y.com"}`))
	w := httptest.NewRecorder()
	h.ClassifyEmail(w, req)
	assertStatus(t, w, http.StatusUnauthorized)
}

func TestClassifyEmail_BadJSON(t *testing.T) {
	h := NewEmailClassificationHandler(&mockEmailClassifier{})
	req := withUser(httptest.NewRequest(http.MethodPost, "/classify-email", strings.NewReader(`{`)))
	w := httptest.NewRecorder()
	h.ClassifyEmail(w, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestClassifyEmail_MissingEmail(t *testing.T) {
	h := NewEmailClassificationHandler(&mockEmailClassifier{})
	req := withUser(httptest.NewRequest(http.MethodPost, "/classify-email", strings.NewReader(`{"email":"   "}`)))
	w := httptest.NewRecorder()
	h.ClassifyEmail(w, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestClassifyEmail_OK(t *testing.T) {
	h := NewEmailClassificationHandler(&mockEmailClassifier{
		fn: func(_ context.Context, email string) emailclassifier.Result {
			return emailclassifier.Result{Email: email, Domain: "acme.com", Category: emailclassifier.CategoryCorporate, Rating: 82, Source: emailclassifier.SourceLLM}
		},
	})
	req := withUser(httptest.NewRequest(http.MethodPost, "/classify-email", strings.NewReader(`{"email":"jane@acme.com"}`)))
	w := httptest.NewRecorder()
	h.ClassifyEmail(w, req)

	assertStatus(t, w, http.StatusOK)
	assertContentType(t, w, "application/json")
	got := decodeJSON[emailclassifier.Result](t, w)
	if got.Category != emailclassifier.CategoryCorporate || got.Rating != 82 {
		t.Fatalf("unexpected: %+v", got)
	}
}
