/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package timeconv

import (
	"testing"
	"time"
)

func TestNow(t *testing.T) {
	now := time.Now()
	ts := Now()
	var drift int64 = 5
	if ts.Seconds < int64(now.Second())-drift {
		t.Errorf("Unexpected time drift: %d", ts.Seconds)
	}
}

func TestTimestamp(t *testing.T) {
	now := time.Now()
	ts := Timestamp(now)

	if now.Unix() != ts.Seconds {
		t.Errorf("Unexpected time drift: %d to %d", now.Second(), ts.Seconds)
	}

	if now.Nanosecond() != int(ts.Nanos) {
		t.Errorf("Unexpected nano drift: %d to %d", now.Nanosecond(), ts.Nanos)
	}
}

func TestTime(t *testing.T) {
	nowts := Now()
	now := Time(nowts)

	if now.Unix() != nowts.Seconds {
		t.Errorf("Unexpected time drift %d", now.Unix())
	}
}

func TestFormat(t *testing.T) {
	now := time.Now()
	nowts := Timestamp(now)

	if now.Format(time.ANSIC) != Format(nowts, time.ANSIC) {
		t.Error("Format mismatch")
	}
}
