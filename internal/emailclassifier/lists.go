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
	"bufio"
	"io"
	"os"
	"strings"
	"sync/atomic"
)

var seedDisposable = map[string]struct{}{
	"mailinator.com": {}, "guerrillamail.com": {}, "10minutemail.com": {},
	"tempmail.com": {}, "trashmail.com": {}, "yopmail.com": {}, "getnada.com": {},
	"throwawaymail.com": {}, "dispostable.com": {}, "maildrop.cc": {},
}

var disposableSet atomic.Pointer[map[string]struct{}]

func init() {
	m := cloneSeed()
	disposableSet.Store(&m)
}

func cloneSeed() map[string]struct{} {
	m := make(map[string]struct{}, len(seedDisposable))
	for k := range seedDisposable {
		m[k] = struct{}{}
	}
	return m
}

// applyDisposable atomically installs seed + entries as the active set.
func applyDisposable(entries map[string]struct{}) int {
	m := cloneSeed()
	for k := range entries {
		m[k] = struct{}{}
	}
	disposableSet.Store(&m)
	return len(m)
}

func isDisposableDomain(domain string) bool {
	if m := disposableSet.Load(); m != nil {
		_, ok := (*m)[domain]
		return ok
	}
	return false
}

var freeProviders = map[string]struct{}{
	"gmail.com": {}, "googlemail.com": {}, "yahoo.com": {}, "ymail.com": {},
	"outlook.com": {}, "hotmail.com": {}, "live.com": {}, "msn.com": {},
	"icloud.com": {}, "me.com": {}, "aol.com": {}, "proton.me": {},
	"protonmail.com": {}, "gmx.com": {}, "zoho.com": {}, "yandex.com": {}, "mail.com": {},
}

var roleLocalParts = map[string]struct{}{
	"admin": {}, "administrator": {}, "info": {}, "support": {}, "sales": {},
	"contact": {}, "help": {}, "billing": {}, "noreply": {}, "no-reply": {},
	"postmaster": {}, "webmaster": {}, "hostmaster": {}, "abuse": {},
	"marketing": {}, "hr": {}, "jobs": {}, "careers": {}, "office": {},
	"test": {}, "testing": {}, "qa": {}, "demo": {}, "example": {}, "sample": {},
	"foo": {}, "bar": {}, "asdf": {}, "user": {}, "dummy": {},
}

func isFreeProvider(domain string) bool {
	_, ok := freeProviders[domain]
	return ok
}

func parseDomains(r io.Reader) map[string]struct{} {
	set := make(map[string]struct{})
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		set[strings.ToLower(line)] = struct{}{}
	}
	return set
}

// LoadDisposable loads a domain file and atomically installs seed + file as the
// active set, returning the number of domains read from the file.
func LoadDisposable(path string) (int, error) {
	f, err := os.Open(path) // #nosec G304 -- operator-provided configuration path
	if err != nil {
		return 0, err
	}
	defer f.Close()
	entries := parseDomains(f)
	applyDisposable(entries)
	return len(entries), nil
}

// LoadFreeProviders merges a domain file into the free-provider set.
func LoadFreeProviders(path string) (int, error) {
	f, err := os.Open(path) // #nosec G304 -- operator-provided configuration path
	if err != nil {
		return 0, err
	}
	defer f.Close()
	entries := parseDomains(f)
	for k := range entries {
		freeProviders[k] = struct{}{}
	}
	return len(entries), nil
}
