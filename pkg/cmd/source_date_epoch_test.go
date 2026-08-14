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

	"github.com/stretchr/testify/require"
)

func TestSourceDateEpochFromEnv(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "1609459200")

	got, err := sourceDateEpochFromEnv()
	require.NoError(t, err, "sourceDateEpochFromEnv()")
	require.NotNil(t, got, "expected non-nil epoch")
	want := time.Unix(1609459200, 0).UTC()
	require.Truef(t, got.Equal(want), "expected %v, got %v", want, *got)
}

func TestSourceDateEpochFromEnvUnset(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "")

	got, err := sourceDateEpochFromEnv()
	require.NoError(t, err, "sourceDateEpochFromEnv()")
	require.Nil(t, got, "expected nil epoch")
}

func TestSourceDateEpochFromEnvInvalid(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "not-a-number")

	_, err := sourceDateEpochFromEnv()
	require.Error(t, err, "expected error for invalid SOURCE_DATE_EPOCH")
}

func TestSourceDateEpochFromEnvNegative(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "-1")

	_, err := sourceDateEpochFromEnv()
	require.Error(t, err, "expected error for negative SOURCE_DATE_EPOCH")
}

func TestSourceDateEpochFromEnvZero(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "0")

	got, err := sourceDateEpochFromEnv()
	require.NoError(t, err, "sourceDateEpochFromEnv() error")
	require.NotNil(t, got, "expected non-nil epoch")
	want := time.Unix(0, 0).UTC()
	require.Truef(t, got.Equal(want), "expected %v, got %v", want, *got)
}
