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

package cmd

import (
	"testing"
	"time"
)

func TestSourceDateEpochFromEnv(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "1609459200")

	got, err := sourceDateEpochFromEnv()
	if err != nil {
		t.Fatalf("sourceDateEpochFromEnv() error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil epoch")
	}
	want := time.Unix(1609459200, 0).UTC()
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, *got)
	}
}

func TestSourceDateEpochFromEnvUnset(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "")

	got, err := sourceDateEpochFromEnv()
	if err != nil {
		t.Fatalf("sourceDateEpochFromEnv() error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil epoch, got %v", *got)
	}
}

func TestSourceDateEpochFromEnvInvalid(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "not-a-number")

	if _, err := sourceDateEpochFromEnv(); err == nil {
		t.Fatal("expected error for invalid SOURCE_DATE_EPOCH")
	}
}
