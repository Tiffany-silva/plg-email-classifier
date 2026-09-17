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
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Refresher periodically fetches the disposable-domain list from a URL,
// installs it into the active set (live, no restart), and persists it to a file
// for fast offline startup. Refreshes fail safe: a network error or a
// suspiciously small response leaves the previous list untouched.
type Refresher struct {
	url      string
	path     string
	interval time.Duration
	minLines int
	http     *http.Client
}

func NewRefresher(url, path string, interval time.Duration) *Refresher {
	return &Refresher{url: url, path: path, interval: interval, minLines: 1000, http: &http.Client{Timeout: 60 * time.Second}}
}

// Start does one immediate refresh, then repeats every interval until ctx is
// cancelled. Intended to run in its own goroutine.
func (r *Refresher) Start(ctx context.Context) {
	if err := r.refreshOnce(ctx); err != nil {
		slog.Warn("email classification: initial disposable-list refresh failed; using existing list", "err", err)
	}
	t := time.NewTicker(r.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := r.refreshOnce(ctx); err != nil {
				slog.Warn("email classification: disposable-list refresh failed; keeping previous list", "err", err)
			}
		}
	}
}

func (r *Refresher) refreshOnce(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return err
	}
	resp, err := r.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return err
	}
	entries := parseDomains(bytes.NewReader(body))
	if len(entries) < r.minLines {
		return fmt.Errorf("refusing update: only %d domains parsed", len(entries))
	}
	total := applyDisposable(entries)
	slog.Info("email classification: disposable list refreshed", "fromSource", len(entries), "activeTotal", total)
	if r.path != "" {
		if err := writeFileAtomic(r.path, body); err != nil {
			slog.Warn("email classification: could not persist disposable list to file", "path", r.path, "err", err)
		}
	}
	return nil
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".disposable-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
