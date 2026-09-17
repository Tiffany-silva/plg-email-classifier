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
	"sync"
	"time"
)

// Cache stores domain-level verdicts. The in-memory implementation below suits
// a single instance; a Redis-backed implementation of this interface can be
// substituted for multi-instance deployments.
type Cache interface {
	Get(domain string) (DomainVerdict, bool)
	Set(domain string, v DomainVerdict)
}

type cacheEntry struct {
	verdict   DomainVerdict
	expiresAt time.Time
}

type MemoryCache struct {
	mu  sync.RWMutex
	m   map[string]cacheEntry
	ttl time.Duration
}

func NewMemoryCache(ttl time.Duration) *MemoryCache {
	return &MemoryCache{m: make(map[string]cacheEntry), ttl: ttl}
}

func (c *MemoryCache) Get(domain string) (DomainVerdict, bool) {
	c.mu.RLock()
	e, ok := c.m[domain]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return DomainVerdict{}, false
	}
	return e.verdict, true
}

func (c *MemoryCache) Set(domain string, v DomainVerdict) {
	c.mu.Lock()
	c.m[domain] = cacheEntry{verdict: v, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}
