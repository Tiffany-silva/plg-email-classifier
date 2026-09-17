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
	"testing"
	"time"
)

func TestSplitComma(t *testing.T) {
	got := splitComma(" a , b ,, c ")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if splitComma("") != nil {
		t.Errorf("empty input should return nil")
	}
}

func TestParseDurationEnv(t *testing.T) {
	t.Setenv("X_DUR", "5s")
	if got := parseDurationEnv("X_DUR", time.Minute); got != 5*time.Second {
		t.Errorf("got %v, want 5s", got)
	}
	if got := parseDurationEnv("X_MISSING", time.Minute); got != time.Minute {
		t.Errorf("missing: got %v, want 1m", got)
	}
	t.Setenv("X_BAD", "notaduration")
	if got := parseDurationEnv("X_BAD", 2*time.Second); got != 2*time.Second {
		t.Errorf("bad: got %v, want default 2s", got)
	}
}

func TestGetbool(t *testing.T) {
	t.Setenv("B_TRUE", "true")
	if !getbool("B_TRUE", false) {
		t.Errorf("want true")
	}
	if !getbool("B_MISSING", true) {
		t.Errorf("missing should use default true")
	}
}
