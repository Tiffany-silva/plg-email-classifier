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

package emailclassifier

import (
	"context"
	"log/slog"
	"net"
	"time"
)

// LLMClassifier is the model-backed classifier used for ambiguous domains.
// internal/anthropic.Client satisfies this via NewAnthropicLLM.
type LLMClassifier interface {
	Classify(ctx context.Context, in Signals, email, domain string) (LLMVerdict, error)
}

// Service runs the full cascade: deterministic rules and cache first, the LLM
// only for genuinely ambiguous domains, then scoring.
type Service struct {
	cache     Cache
	llm       LLMClassifier
	resolver  *net.Resolver
	mxTimeout time.Duration
}

// NewService constructs a Service. A nil llm disables the model path: unknown
// reachable domains resolve to "unknown" instead of calling the API.
func NewService(cache Cache, llm LLMClassifier, mxTimeout time.Duration) *Service {
	return &Service{cache: cache, llm: llm, resolver: net.DefaultResolver, mxTimeout: mxTimeout}
}

// Classify returns a full result for a single email address. It never returns
// an error: failures degrade to "unknown" so a signup is never blocked.
func (s *Service) Classify(ctx context.Context, rawEmail string) Result {
	email := normalize(rawEmail)

	if !validSyntax(email) {
		return Result{
			Email:     rawEmail,
			Category:  CategoryInvalid,
			Source:    SourceRules,
			Signals:   Signals{SyntaxValid: false},
			Reasoning: "Address failed basic syntax validation.",
		}
	}

	local, domain, _ := parse(email)
	roleBased := isRoleBased(local)

	verdict, cached := s.cache.Get(domain)
	if cached {
		verdict.Source = SourceCache
	} else {
		verdict = s.resolveDomain(ctx, email, domain)
		// invalid and unknown are transient; never cache them.
		if verdict.Category != CategoryInvalid && verdict.Category != CategoryUnknown {
			s.cache.Set(domain, verdict)
		}
	}

	finalCat := applyLocalPartOverride(verdict.Category, roleBased)
	signals := Signals{
		SyntaxValid:  true,
		MXValid:      verdict.MXValid,
		Disposable:   verdict.Disposable,
		FreeProvider: verdict.FreeProvider,
		RoleBased:    roleBased,
	}
	reasoning := verdict.Reasoning
	if finalCat != verdict.Category {
		reasoning = "Role-based / testing mailbox on a valid provider."
	}

	return Result{
		Email:      rawEmail,
		Domain:     domain,
		Category:   finalCat,
		Rating:     rate(finalCat, verdict.Confidence, signals),
		Confidence: verdict.Confidence,
		Source:     verdict.Source,
		Signals:    signals,
		Reasoning:  reasoning,
	}
}

func (s *Service) resolveDomain(ctx context.Context, email, domain string) DomainVerdict {
	if isDisposableDomain(domain) {
		return DomainVerdict{
			Domain: domain, Category: CategoryDisposable, Confidence: 0.98,
			Disposable: true, MXValid: true, Source: SourceRules,
			Reasoning: "Domain is a known disposable email provider.",
		}
	}
	if isFreeProvider(domain) {
		return DomainVerdict{
			Domain: domain, Category: CategoryPersonal, Confidence: 0.95,
			FreeProvider: true, MXValid: true, Source: SourceRules,
			Reasoning: "Domain is a known consumer email provider.",
		}
	}
	if !lookupMX(ctx, s.resolver, domain, s.mxTimeout) {
		return DomainVerdict{
			Domain: domain, Category: CategoryInvalid, Confidence: 0.9,
			MXValid: false, Source: SourceRules,
			Reasoning: "Domain has no mail exchanger and cannot receive email.",
		}
	}

	// Unknown but reachable -> the only path that may spend an API call.
	if s.llm == nil {
		return DomainVerdict{
			Domain: domain, Category: CategoryUnknown, Confidence: 0.3,
			MXValid: true, Source: SourceRules,
			Reasoning: "Reachable domain; classifier model disabled, left unclassified for review.",
		}
	}

	sig := Signals{SyntaxValid: true, MXValid: true}
	out, err := s.llm.Classify(ctx, sig, email, domain)
	if err != nil {
		slog.WarnContext(ctx, "email classification: llm call failed, leaving domain unclassified", "domain", domain, "err", err)
		return DomainVerdict{
			Domain: domain, Category: CategoryUnknown, Confidence: 0.3,
			MXValid: true, Source: SourceRules,
			Reasoning: "Reachable domain; model unavailable, left unclassified for review.",
		}
	}
	return DomainVerdict{
		Domain: domain, Category: out.Category, Confidence: out.Confidence,
		MXValid: true, Source: SourceLLM, Reasoning: out.Reasoning,
	}
}
