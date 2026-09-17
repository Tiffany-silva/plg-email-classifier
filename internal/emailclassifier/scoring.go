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

func rate(cat Category, confidence float64, _ Signals) int {
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}
	var base, span int
	switch cat {
	case CategoryCorporate:
		base, span = 70, 30
	case CategoryPersonal:
		base, span = 30, 30
	case CategoryProviderTesting:
		base, span = 10, 20
	case CategoryDisposable:
		base, span = 0, 10
	case CategoryUnknown:
		base, span = 15, 20
	default:
		return 0
	}
	score := base + int(confidence*float64(span))
	if score > base+span {
		score = base + span
	}
	return score
}

func applyLocalPartOverride(cat Category, roleBased bool) Category {
	if roleBased && cat == CategoryPersonal {
		return CategoryProviderTesting
	}
	return cat
}
