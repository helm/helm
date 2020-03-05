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

package gitutil

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"testing"
)

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

// TestHelperProcess is not a test. It is used to create a mock process to spawn as a child process for testing exec.Command().
// Borrowed from the way Go tests exec.Command() internally: https://github.com/golang/go/blob/master/src/os/exec/exec_test.go#L727
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	cmd := args[len(args)-1] // the last arg is the github url. using this to determine what to return.
	result := "Unknown Command"
	exitCode := 1

	switch cmd {
	case "success":
		exitCode = 0
		result = `From git@github.com:helm/helm.git
9b42702a4bced339ff424a78ad68dd6be6e1a80a       refs/heads/dev
9668ad4d90c5e95bd520e58e7387607be6b63bb6       refs/heads/master
44fb06eb69fecd4b6a5b2443a4768ba12bd70c09       refs/tags/v2.10.0
9ad53aac42165a5fadc6c87be0dea6b115f93090       refs/tags/v2.10.0^{}
4fdd07f21418abb43925998cf690857adc16451b       refs/tags/v2.10.0-rc.1
aa98e7e3dd2356bce72e8e367e8c87e8085c692b       refs/tags/v2.10.0-rc.1^{}`

	case "error":
		exitCode = 1
		result = `ssh: Could not resolve hostname git: nodename nor servname provided, or not known
fatal: Could not read from remote repository.

Please make sure you have the correct access rights
and the repository exists.`
	}

	fmt.Fprint(os.Stdout, result)
	os.Exit(exitCode)
}

func TestGetRefsWhenGitReturnsRefs(t *testing.T) {
	expected := map[string]string{
		"dev":          "9b42702a4bced339ff424a78ad68dd6be6e1a80a",
		"master":       "9668ad4d90c5e95bd520e58e7387607be6b63bb6",
		"v2.10.0":      "9ad53aac42165a5fadc6c87be0dea6b115f93090",
		"v2.10.0-rc.1": "aa98e7e3dd2356bce72e8e367e8c87e8085c692b",
	}
	execCommand = fakeExecCommand
	defer func() { execCommand = exec.Command }()
	result, err := GetRefs("success")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}
func TestGetRefsWhenGitCommandReturnsError(t *testing.T) {
	execCommand = fakeExecCommand
	defer func() { execCommand = exec.Command }()
	_, err := GetRefs("error")
	if err == nil {
		t.Errorf("Error should have been returned, but was not")
	}
}
