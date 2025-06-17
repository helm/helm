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

package time

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	timeParseString = `"1977-09-02T22:04:05Z"`
	timeString      = "1977-09-02 22:04:05 +0000 UTC"
)

func givenTime(t *testing.T) Time {
	t.Helper()
	result, err := Parse(time.RFC3339, "1977-09-02T22:04:05Z")
	require.NoError(t, err)
	return result
}

func TestDate(t *testing.T) {
	testingTime := givenTime(t)
	got := Date(1977, 9, 2, 22, 04, 05, 0, time.UTC)
	assert.Equal(t, timeString, got.String())
	assert.True(t, testingTime.Equal(got))
	assert.True(t, got.Equal(testingTime))
}

func TestNow(t *testing.T) {
	testingTime := givenTime(t)
	got := Now()
	assert.True(t, testingTime.Before(got))
	assert.True(t, got.After(testingTime))
}

func TestTime_Add(t *testing.T) {
	testingTime := givenTime(t)
	got := testingTime.Add(time.Hour)
	assert.Equal(t, timeString, testingTime.String())
	assert.Equal(t, "1977-09-02 23:04:05 +0000 UTC", got.String())
}

func TestTime_AddDate(t *testing.T) {
	testingTime := givenTime(t)
	got := testingTime.AddDate(1, 1, 1)
	assert.Equal(t, "1978-10-03 22:04:05 +0000 UTC", got.String())
}

func TestTime_In(t *testing.T) {
	testingTime := givenTime(t)
	edt, err := time.LoadLocation("America/New_York")
	assert.NoError(t, err)
	got := testingTime.In(edt)
	assert.Equal(t, "America/New_York", got.Location().String())
}

func TestTime_MarshalJSONNonZero(t *testing.T) {
	testingTime := givenTime(t)
	res, err := json.Marshal(testingTime)
	assert.NoError(t, err)
	assert.Equal(t, timeParseString, string(res))
}

func TestTime_MarshalJSONZeroValue(t *testing.T) {
	res, err := json.Marshal(Time{})
	assert.NoError(t, err)
	assert.Equal(t, `""`, string(res))
}

func TestTime_Round(t *testing.T) {
	testingTime := givenTime(t)
	got := testingTime.Round(time.Hour)
	assert.Equal(t, timeString, testingTime.String())
	assert.Equal(t, "1977-09-02 22:00:00 +0000 UTC", got.String())
}

func TestTime_Sub(t *testing.T) {
	testingTime := givenTime(t)
	before, err := Parse(time.RFC3339, "1977-09-01T22:04:05Z")
	require.NoError(t, err)
	got := testingTime.Sub(before)
	assert.Equal(t, "24h0m0s", got.String())
}

func TestTime_Truncate(t *testing.T) {
	testingTime := givenTime(t)
	got := testingTime.Truncate(time.Hour)
	assert.Equal(t, timeString, testingTime.String())
	assert.Equal(t, "1977-09-02 22:00:00 +0000 UTC", got.String())
}

func TestTime_UTC(t *testing.T) {
	edtTime, err := Parse(time.RFC3339, "1977-09-03T05:04:05+07:00")
	require.NoError(t, err)
	got := edtTime.UTC()
	assert.Equal(t, timeString, got.String())
}

func TestTime_UnmarshalJSONNonZeroValue(t *testing.T) {
	testingTime := givenTime(t)
	var myTime Time
	err := json.Unmarshal([]byte(timeParseString), &myTime)
	assert.NoError(t, err)
	assert.True(t, testingTime.Equal(myTime))
}

func TestTime_UnmarshalJSONEmptyString(t *testing.T) {
	var myTime Time
	err := json.Unmarshal([]byte(emptyString), &myTime)
	assert.NoError(t, err)
	assert.True(t, myTime.IsZero())
}

func TestTime_UnmarshalJSONNullString(t *testing.T) {
	var myTime Time
	err := json.Unmarshal([]byte("null"), &myTime)
	assert.NoError(t, err)
	assert.True(t, myTime.IsZero())
}

func TestTime_UnmarshalJSONZeroValue(t *testing.T) {
	// This test ensures that we can unmarshal any time value that was output
	// with the current go default value of "0001-01-01T00:00:00Z"
	var myTime Time
	err := json.Unmarshal([]byte(`"0001-01-01T00:00:00Z"`), &myTime)
	assert.NoError(t, err)
	assert.True(t, myTime.IsZero())
}

func TestUnix(t *testing.T) {
	got := Unix(242085845, 0)
	assert.Equal(t, int64(242085845), got.Unix())
	assert.Equal(t, timeString, got.UTC().String())
}
