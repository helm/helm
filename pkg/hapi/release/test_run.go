package release

import "time"

type TestRunStatus int

const (
	TestRun_UNKNOWN TestRunStatus = iota
	TestRun_SUCCESS
	TestRun_FAILURE
	TestRun_RUNNING
)

var testRunStatusNames = [...]string{
	"UNKNOWN",
	"SUCCESS",
	"FAILURE",
	"RUNNING",
}

func (x TestRunStatus) String() string { return testRunStatusNames[x] }

type TestRun struct {
	Name        string        `json:"name,omitempty"`
	Status      TestRunStatus `json:"status,omitempty"`
	Info        string        `json:"info,omitempty"`
	StartedAt   time.Time     `json:"started_at,omitempty"`
	CompletedAt time.Time     `json:"completed_at,omitempty"`
}
