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
	"net"
	"regexp"
	"strings"
	"time"
)

var emailRe = regexp.MustCompile(`^[a-zA-Z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$`)

func normalize(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

func parse(email string) (local, domain string, ok bool) {
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return "", "", false
	}
	return email[:at], email[at+1:], true
}

func validSyntax(email string) bool { return len(email) <= 254 && emailRe.MatchString(email) }

func isRoleBased(local string) bool {
	base := local
	if i := strings.IndexByte(base, '+'); i >= 0 {
		base = base[:i]
	}
	if _, ok := roleLocalParts[base]; ok {
		return true
	}
	if _, ok := roleLocalParts[strings.TrimRight(base, "0123456789-_.")]; ok {
		return true
	}
	return false
}

func lookupMX(ctx context.Context, resolver *net.Resolver, domain string, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if mx, err := resolver.LookupMX(ctx, domain); err == nil && len(mx) > 0 {
		return true
	}
	if addrs, err := resolver.LookupHost(ctx, domain); err == nil && len(addrs) > 0 {
		return true
	}
	return false
}
