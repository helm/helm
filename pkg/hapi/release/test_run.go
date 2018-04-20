/*
Copyright 2018 The Kubernetes Authors All rights reserved.
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
