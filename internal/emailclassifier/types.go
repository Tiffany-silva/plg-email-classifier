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

// Package emailclassifier classifies a signup email into a category and a
// 0-100 lead-quality rating, using a cheap-to-expensive cascade: deterministic
// rules and a domain cache first, the Anthropic API only for ambiguous domains.
// Responses are portal-owned, so all JSON fields are camelCase.
package emailclassifier

type Category string

const (
	CategoryCorporate       Category = "corporate"
	CategoryDisposable      Category = "disposable"
	CategoryProviderTesting Category = "provider_testing"
	CategoryPersonal        Category = "personal"
	CategoryInvalid         Category = "invalid"
	CategoryUnknown         Category = "unknown"
)

type Source string

const (
	SourceRules Source = "rules"
	SourceCache Source = "cache"
	SourceLLM   Source = "llm"
)

type Signals struct {
	SyntaxValid  bool `json:"syntaxValid"`
	MXValid      bool `json:"mxValid"`
	Disposable   bool `json:"disposable"`
	FreeProvider bool `json:"freeProvider"`
	RoleBased    bool `json:"roleBased"`
}

type Result struct {
	Email      string   `json:"email"`
	Domain     string   `json:"domain"`
	Category   Category `json:"category"`
	Rating     int      `json:"rating"`
	Confidence float64  `json:"confidence"`
	Source     Source   `json:"source"`
	Signals    Signals  `json:"signals"`
	Reasoning  string   `json:"reasoning"`
}

type DomainVerdict struct {
	Domain       string
	Category     Category
	Confidence   float64
	Reasoning    string
	MXValid      bool
	Disposable   bool
	FreeProvider bool
	Source       Source
}

type LLMVerdict struct {
	Category   Category
	Confidence float64
	Reasoning  string
}
