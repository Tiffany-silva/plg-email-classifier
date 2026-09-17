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
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/plg-email-classifier/internal/emailclassifier"
	"github.com/wso2-open-operations/plg-email-classifier/internal/middleware"
)

// emailClassifier abstracts the classification service used by the handler,
// allowing it to be tested without the real cascade or network access.
type emailClassifier interface {
	Classify(ctx context.Context, rawEmail string) emailclassifier.Result
}

// EmailClassificationHandler handles email-classification requests.
type EmailClassificationHandler struct {
	classifier emailClassifier
}

func NewEmailClassificationHandler(classifier emailClassifier) *EmailClassificationHandler {
	return &EmailClassificationHandler{classifier: classifier}
}

type classifyEmailRequest struct {
	Email string `json:"email"`
}

// ClassifyEmail handles POST /classify-email.
func (h *EmailClassificationHandler) ClassifyEmail(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	var payload classifyEmailRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if strings.TrimSpace(payload.Email) == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	// Classification never fails the request: on any model/network problem the
	// service degrades to an "unknown" verdict rather than erroring, so there is
	// deliberately no upstream-error mapping here.
	result := h.classifier.Classify(r.Context(), payload.Email)
	writeJSONValue(w, http.StatusOK, result)
}
