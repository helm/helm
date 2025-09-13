/*
Copyright The Helm Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package registry

import (
	"strings"
	"testing"
)

func TestGetTagMatchingVersionOrConstraint_ExactMatch(t *testing.T) {
	tags := []string{"1.0.0", "1.2.3", "2.0.0"}
	got, err := GetTagMatchingVersionOrConstraint(tags, "1.2.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.2.3" {
		t.Fatalf("expected exact match '1.2.3', got %q", got)
	}
}

func TestGetTagMatchingVersionOrConstraint_EmptyVersionWildcard(t *testing.T) {
	// Includes a non-semver tag which should be skipped
	tags := []string{"latest", "0.9.0", "1.0.0"}
	got, err := GetTagMatchingVersionOrConstraint(tags, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should pick the first valid semver tag in order, which is 0.9.0
	if got != "0.9.0" {
		t.Fatalf("expected '0.9.0', got %q", got)
	}
}

func TestGetTagMatchingVersionOrConstraint_ConstraintRange(t *testing.T) {
	tags := []string{"0.5.0", "1.0.0", "1.1.0", "2.0.0"}

	// Caret range
	got, err := GetTagMatchingVersionOrConstraint(tags, "^1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.0.0" { // first match in order
		t.Fatalf("expected '1.0.0', got %q", got)
	}

	// Compound range
	got, err = GetTagMatchingVersionOrConstraint(tags, ">=1.0.0 <2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.0.0" {
		t.Fatalf("expected '1.0.0', got %q", got)
	}
}

func TestGetTagMatchingVersionOrConstraint_InvalidConstraint(t *testing.T) {
	tags := []string{"1.0.0"}
	_, err := GetTagMatchingVersionOrConstraint(tags, ">a1")
	if err == nil {
		t.Fatalf("expected error for invalid constraint")
	}
}

func TestGetTagMatchingVersionOrConstraint_NoMatches(t *testing.T) {
	tags := []string{"0.1.0", "0.2.0"}
	_, err := GetTagMatchingVersionOrConstraint(tags, ">=1.0.0")
	if err == nil {
		t.Fatalf("expected error when no tags match")
	}
	if !strings.Contains(err.Error(), ">=1.0.0") {
		t.Fatalf("expected error to contain version string, got: %v", err)
	}
}

func TestGetTagMatchingVersionOrConstraint_SkipsNonSemverTags(t *testing.T) {
	tags := []string{"alpha", "1.0.0", "beta", "1.1.0"}
	got, err := GetTagMatchingVersionOrConstraint(tags, ">=1.0.0 <2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.0.0" {
		t.Fatalf("expected '1.0.0', got %q", got)
	}
}

func TestGetTagMatchingVersionOrConstraint_OrderMatters_FirstMatchReturned(t *testing.T) {
	// Both 1.2.0 and 1.3.0 satisfy >=1.2.0 <2.0.0, but the function returns the first in input order
	tags := []string{"1.3.0", "1.2.0"}
	got, err := GetTagMatchingVersionOrConstraint(tags, ">=1.2.0 <2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.3.0" {
		t.Fatalf("expected '1.3.0' (first satisfying tag), got %q", got)
	}
}

func TestGetTagMatchingVersionOrConstraint_ExactMatchHasPrecedence(t *testing.T) {
	// Exact match should be returned even if another earlier tag would match the parsed constraint
	tags := []string{"1.3.0", "1.2.3"}
	got, err := GetTagMatchingVersionOrConstraint(tags, "1.2.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.2.3" {
		t.Fatalf("expected exact match '1.2.3', got %q", got)
	}
}
