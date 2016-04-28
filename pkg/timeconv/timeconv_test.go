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
