package release

import google_protobuf "github.com/golang/protobuf/ptypes/timestamp"

// TestSuite comprises of the last run of the pre-defined test suite of a release version
type TestSuite struct {
	// StartedAt indicates the date/time this test suite was kicked off
	StartedAt *google_protobuf.Timestamp `json:"started_at,omitempty"`
	// CompletedAt indicates the date/time this test suite was completed
	CompletedAt *google_protobuf.Timestamp `json:"completed_at,omitempty"`
	// Results are the results of each segment of the test
	Results []*TestRun `json:"results,omitempty"`
}
