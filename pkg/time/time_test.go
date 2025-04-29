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
	result, err := Parse(time.RFC3339, timeParseString)
	require.NoError(t, err)
	return result
}

func TestDate(t *testing.T) {
	got := Date(1977, 9, 2, 22, 04, 05, 0, time.UTC)
	assert.Equal(t, timeString, got.String())
}

func TestNow(t *testing.T) {
	testingTime := givenTime(t)
	got := Now()
	assert.Truef(t, testingTime.Before(got), "expected %s before %s", testingTime.String(), got.String())
}

func TestParse(t *testing.T) {
	testingTime := givenTime(t)
	got, err := Parse(time.RFC3339, timeParseString)
	assert.NoError(t, err)
	if testingTime.Before(got) {
		t.Errorf("expected %s before %s", testingTime.String(), got.String())
	}
}

//func TestParseInLocation(t *testing.T) {
//
//	got, err := ParseInLocation(tt.args.layout, tt.args.value, tt.args.loc)
//	if (err != nil) != tt.wantErr {
//		t.Errorf("ParseInLocation() error = %v, wantErr %v", err, tt.wantErr)
//		return
//	}
//	if !reflect.DeepEqual(got, tt.want) {
//		t.Errorf("ParseInLocation() got = %v, want %v", got, tt.want)
//	}
//}

//func TestTime_Add(t *testing.T) {
//
//	sut := Time{
//		Time: tt.fields.Time,
//	}
//	if got := t.Add(tt.args.d); !reflect.DeepEqual(got, tt.want) {
//		t.Errorf("Add() = %v, want %v", got, tt.want)
//	}
//}
//
//func TestTime_AddDate(t *testing.T) {
//
//	sut := Time{
//		Time: tt.fields.Time,
//	}
//	if got := t.AddDate(tt.args.years, tt.args.months, tt.args.days); !reflect.DeepEqual(got, tt.want) {
//		t.Errorf("AddDate() = %v, want %v", got, tt.want)
//	}
//}

//func TestTime_After(t *testing.T) {
//
//	sut := Time{
//		Time: tt.fields.Time,
//	}
//	if got := t.After(tt.args.u); got != tt.want {
//		t.Errorf("After() = %v, want %v", got, tt.want)
//	}
//
//}
//
//func TestTime_Before(t *testing.T) {
//
//	sut := Time{
//		Time: tt.fields.Time,
//	}
//	if got := t.Before(tt.args.u); got != tt.want {
//		t.Errorf("Before() = %v, want %v", got, tt.want)
//	}
//
//}
//
//func TestTime_Equal(t *testing.T) {
//
//	sut := Time{
//		Time: tt.fields.Time,
//	}
//	if got := t.Equal(tt.args.u); got != tt.want {
//		t.Errorf("Equal() = %v, want %v", got, tt.want)
//	}
//
//}

//func TestTime_In(t *testing.T) {
//
//	sut := Time{
//		Time: tt.fields.Time,
//	}
//	if got := t.In(tt.args.loc); !reflect.DeepEqual(got, tt.want) {
//		t.Errorf("In() = %v, want %v", got, tt.want)
//	}
//
//}
//
//func TestTime_Local(t *testing.T) {
//
//	sut := Time{
//		Time: tt.fields.Time,
//	}
//	if got := t.Local(); !reflect.DeepEqual(got, tt.want) {
//		t.Errorf("Local() = %v, want %v", got, tt.want)
//	}
//
//}

func TestTime_MarshalJSONNonZero(t *testing.T) {
	testingTime := givenTime(t)
	res, err := json.Marshal(testingTime)
	if err != nil {
		t.Fatal(err)
	}
	if timeParseString != string(res) {
		t.Errorf("expected a marshaled value of %s, got %s", timeParseString, res)
	}
}

func TestTime_MarshalJSONZeroValue(t *testing.T) {
	res, err := json.Marshal(Time{})
	if err != nil {
		t.Fatal(err)
	}
	if string(res) != emptyString {
		t.Errorf("expected zero value to marshal to empty string, got %s", res)
	}
}

//func TestTime_Round(t *testing.T) {
//	if got := t.Round(tt.args.d); !reflect.DeepEqual(got, tt.want) {
//		t.Errorf("Round() = %v, want %v", got, tt.want)
//	}
//}

//func TestTime_Sub(t *testing.T) {
//	sut := Time{
//		Time: tt.fields.Time,
//	}
//	if got := t.Sub(tt.args.u); got != tt.want {
//		t.Errorf("Sub() = %v, want %v", got, tt.want)
//	}
//}

//func TestTime_Truncate(t *testing.T) {
//
//	sut := Time{
//		Time: tt.fields.Time,
//	}
//	if got := t.Truncate(tt.args.d); !reflect.DeepEqual(got, tt.want) {
//		t.Errorf("Truncate() = %v, want %v", got, tt.want)
//	}
//
//}
//
//func TestTime_UTC(t *testing.T) {
//
//	sut := Time{
//		Time: tt.fields.Time,
//	}
//	if got := t.UTC(); !reflect.DeepEqual(got, tt.want) {
//		t.Errorf("UTC() = %v, want %v", got, tt.want)
//	}
//
//}

func TestTime_UnmarshalJSONNonZeroValue(t *testing.T) {
	testingTime := givenTime(t)
	var myTime Time
	err := json.Unmarshal([]byte(timeParseString), &myTime)
	if err != nil {
		t.Fatal(err)
	}
	if !myTime.Equal(testingTime) {
		t.Errorf("expected time to be equal to %v, got %v", testingTime, myTime)
	}
}

func TestTime_UnmarshalJSONEmptyString(t *testing.T) {
	var myTime Time
	err := json.Unmarshal([]byte(emptyString), &myTime)
	if err != nil {
		t.Fatal(err)
	}
	if !myTime.IsZero() {
		t.Errorf("expected time to be equal to zero value, got %v", myTime)
	}
}

func TestTime_UnmarshalJSONZeroValue(t *testing.T) {
	// This test ensures that we can unmarshal any time value that was output
	// with the current go default value of "0001-01-01T00:00:00Z"
	var myTime Time
	err := json.Unmarshal([]byte(`"0001-01-01T00:00:00Z"`), &myTime)
	if err != nil {
		t.Fatal(err)
	}
	if !myTime.IsZero() {
		t.Errorf("expected time to be equal to zero value, got %v", myTime)
	}
}

//func TestUnix(t *testing.T) {
//
//	if got := Unix(tt.args.sec, tt.args.nsec); !reflect.DeepEqual(got, tt.want) {
//		t.Errorf("Unix() = %v, want %v", got, tt.want)
//	}
//
//}
