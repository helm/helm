package release

import google_protobuf "github.com/golang/protobuf/ptypes/timestamp"

type TestRun_Status int32

const (
	TestRun_UNKNOWN TestRun_Status = 0
	TestRun_SUCCESS TestRun_Status = 1
	TestRun_FAILURE TestRun_Status = 2
	TestRun_RUNNING TestRun_Status = 3
)

var TestRun_Status_name = map[int32]string{
	0: "UNKNOWN",
	1: "SUCCESS",
	2: "FAILURE",
	3: "RUNNING",
}
var TestRun_Status_value = map[string]int32{
	"UNKNOWN": 0,
	"SUCCESS": 1,
	"FAILURE": 2,
	"RUNNING": 3,
}

func (x TestRun_Status) String() string {
	return TestRun_Status_name[int32(x)]
}

type TestRun struct {
	Name        string                     `json:"name,omitempty"`
	Status      TestRun_Status             `json:"status,omitempty"`
	Info        string                     `json:"info,omitempty"`
	StartedAt   *google_protobuf.Timestamp `json:"started_at,omitempty"`
	CompletedAt *google_protobuf.Timestamp `json:"completed_at,omitempty"`
}
