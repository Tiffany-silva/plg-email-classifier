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
	"testing"
	"time"
)

type alwaysMissCache struct{}

func (alwaysMissCache) Get(string) (DomainVerdict, bool) { return DomainVerdict{}, false }
func (alwaysMissCache) Set(string, DomainVerdict)        {}

func newTestService() *Service { return NewService(alwaysMissCache{}, nil, time.Second) }

func TestSyntax(t *testing.T) {
	r := newTestService().Classify(context.Background(), "not-an-email")
	if r.Category != CategoryInvalid || r.Rating != 0 {
		t.Fatalf("got %s/%d, want invalid/0", r.Category, r.Rating)
	}
}

func TestDisposable(t *testing.T) {
	r := newTestService().Classify(context.Background(), "jane@mailinator.com")
	if r.Category != CategoryDisposable || r.Rating > 10 || r.Source != SourceRules {
		t.Fatalf("got %s rating=%d source=%s", r.Category, r.Rating, r.Source)
	}
}

func TestPersonalVsTesting(t *testing.T) {
	s := newTestService()
	if got := s.Classify(context.Background(), "jane.doe@gmail.com"); got.Category != CategoryPersonal {
		t.Fatalf("got %s, want personal", got.Category)
	}
	tst := s.Classify(context.Background(), "test@gmail.com")
	if tst.Category != CategoryProviderTesting || !tst.Signals.RoleBased {
		t.Fatalf("got %s roleBased=%t", tst.Category, tst.Signals.RoleBased)
	}
}

func TestUnknownWhenLLMDisabled(t *testing.T) {
	r := newTestService().Classify(context.Background(), "jane@example-unknown-corp-xyz.com")
	if r.Category != CategoryUnknown && r.Category != CategoryInvalid {
		t.Fatalf("got %s, want unknown/invalid", r.Category)
	}
	if r.Category == CategoryUnknown && r.Rating > 35 {
		t.Fatalf("unknown rating too high: %d", r.Rating)
	}
}

type fakeLLM struct {
	v   LLMVerdict
	err error
}

func (f fakeLLM) Classify(context.Context, Signals, string, string) (LLMVerdict, error) {
	return f.v, f.err
}

func TestScoringBands(t *testing.T) {
	cases := []struct {
		cat  Category
		conf float64
		want int
	}{
		{CategoryCorporate, 1, 100}, {CategoryCorporate, 0, 70},
		{CategoryDisposable, 1, 10}, {CategoryUnknown, 0, 15}, {CategoryInvalid, 1, 0},
	}
	for _, c := range cases {
		if got := rate(c.cat, c.conf, Signals{}); got != c.want {
			t.Errorf("rate(%s,%v)=%d want %d", c.cat, c.conf, got, c.want)
		}
	}
}

var _ LLMClassifier = fakeLLM{} // fakeLLM satisfies the interface
