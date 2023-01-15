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

package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
)

func TestSearchHubCmd(t *testing.T) {

	// Setup a mock search service
	var searchResult = `{"data":[{"id":"stable/phpmyadmin","type":"chart","attributes":{"name":"phpmyadmin","repo":{"name":"stable","url":"https://charts.helm.sh/stable"},"description":"phpMyAdmin is an mysql administration frontend","home":"https://www.phpmyadmin.net/","keywords":["mariadb","mysql","phpmyadmin"],"maintainers":[{"name":"Bitnami","email":"containers@bitnami.com"}],"sources":["https://github.com/bitnami/bitnami-docker-phpmyadmin"],"icon":""},"links":{"self":"/v1/charts/stable/phpmyadmin"},"relationships":{"latestChartVersion":{"data":{"version":"3.0.0","app_version":"4.9.0-1","created":"2019-08-08T17:57:31.38Z","digest":"119c499251bffd4b06ff0cd5ac98c2ce32231f84899fb4825be6c2d90971c742","urls":["https://charts.helm.sh/stable/phpmyadmin-3.0.0.tgz"],"readme":"/v1/assets/stable/phpmyadmin/versions/3.0.0/README.md","values":"/v1/assets/stable/phpmyadmin/versions/3.0.0/values.yaml"},"links":{"self":"/v1/charts/stable/phpmyadmin/versions/3.0.0"}}}},{"id":"bitnami/phpmyadmin","type":"chart","attributes":{"name":"phpmyadmin","repo":{"name":"bitnami","url":"https://charts.bitnami.com"},"description":"phpMyAdmin is an mysql administration frontend","home":"https://www.phpmyadmin.net/","keywords":["mariadb","mysql","phpmyadmin"],"maintainers":[{"name":"Bitnami","email":"containers@bitnami.com"}],"sources":["https://github.com/bitnami/bitnami-docker-phpmyadmin"],"icon":""},"links":{"self":"/v1/charts/bitnami/phpmyadmin"},"relationships":{"latestChartVersion":{"data":{"version":"3.0.0","app_version":"4.9.0-1","created":"2019-08-08T18:34:13.341Z","digest":"66d77cf6d8c2b52c488d0a294cd4996bd5bad8dc41d3829c394498fb401c008a","urls":["https://charts.bitnami.com/bitnami/phpmyadmin-3.0.0.tgz"],"readme":"/v1/assets/bitnami/phpmyadmin/versions/3.0.0/README.md","values":"/v1/assets/bitnami/phpmyadmin/versions/3.0.0/values.yaml"},"links":{"self":"/v1/charts/bitnami/phpmyadmin/versions/3.0.0"}}}}]}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, searchResult)
	}))
	defer ts.Close()

	// The expected output has the URL to the mocked search service in it
	// Trailing spaces are necessary to preserve in "expected" as the uitable package adds
	// them during printing.
	var expected = fmt.Sprintf(`URL                                             	CHART VERSION	APP VERSION	DESCRIPTION                                   
%s/charts/stable/phpmyadmin 	3.0.0        	4.9.0-1    	phpMyAdmin is an mysql administration frontend
%s/charts/bitnami/phpmyadmin	3.0.0        	4.9.0-1    	phpMyAdmin is an mysql administration frontend
`, ts.URL, ts.URL)

	testcmd := "search hub --endpoint " + ts.URL + " maria"
	storage := storageFixture()
	_, out, err := executeActionCommandC(storage, testcmd)
	if err != nil {
		t.Errorf("unexpected error, %s", err)
	}
	if out != expected {
		t.Error("expected and actual output did not match")
		t.Log(out)
		t.Log(expected)
	}
}

func TestSearchHubListRepoCmd(t *testing.T) {

	// Setup a mock search service
	var searchResult = `{"data":[{"id":"stable/phpmyadmin","type":"chart","attributes":{"name":"phpmyadmin","repo":{"name":"stable","url":"https://charts.helm.sh/stable"},"description":"phpMyAdmin is an mysql administration frontend","home":"https://www.phpmyadmin.net/","keywords":["mariadb","mysql","phpmyadmin"],"maintainers":[{"name":"Bitnami","email":"containers@bitnami.com"}],"sources":["https://github.com/bitnami/bitnami-docker-phpmyadmin"],"icon":""},"links":{"self":"/v1/charts/stable/phpmyadmin"},"relationships":{"latestChartVersion":{"data":{"version":"3.0.0","app_version":"4.9.0-1","created":"2019-08-08T17:57:31.38Z","digest":"119c499251bffd4b06ff0cd5ac98c2ce32231f84899fb4825be6c2d90971c742","urls":["https://charts.helm.sh/stable/phpmyadmin-3.0.0.tgz"],"readme":"/v1/assets/stable/phpmyadmin/versions/3.0.0/README.md","values":"/v1/assets/stable/phpmyadmin/versions/3.0.0/values.yaml"},"links":{"self":"/v1/charts/stable/phpmyadmin/versions/3.0.0"}}}},{"id":"bitnami/phpmyadmin","type":"chart","attributes":{"name":"phpmyadmin","repo":{"name":"bitnami","url":"https://charts.bitnami.com"},"description":"phpMyAdmin is an mysql administration frontend","home":"https://www.phpmyadmin.net/","keywords":["mariadb","mysql","phpmyadmin"],"maintainers":[{"name":"Bitnami","email":"containers@bitnami.com"}],"sources":["https://github.com/bitnami/bitnami-docker-phpmyadmin"],"icon":""},"links":{"self":"/v1/charts/bitnami/phpmyadmin"},"relationships":{"latestChartVersion":{"data":{"version":"3.0.0","app_version":"4.9.0-1","created":"2019-08-08T18:34:13.341Z","digest":"66d77cf6d8c2b52c488d0a294cd4996bd5bad8dc41d3829c394498fb401c008a","urls":["https://charts.bitnami.com/bitnami/phpmyadmin-3.0.0.tgz"],"readme":"/v1/assets/bitnami/phpmyadmin/versions/3.0.0/README.md","values":"/v1/assets/bitnami/phpmyadmin/versions/3.0.0/values.yaml"},"links":{"self":"/v1/charts/bitnami/phpmyadmin/versions/3.0.0"}}}}]}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, searchResult)
	}))
	defer ts.Close()

	// The expected output has the URL to the mocked search service in it
	// Trailing spaces are necessary to preserve in "expected" as the uitable package adds
	// them during printing.
	var expected = fmt.Sprintf(`URL                                             	CHART VERSION	APP VERSION	DESCRIPTION                                   	REPO URL                     
%s/charts/stable/phpmyadmin 	3.0.0        	4.9.0-1    	phpMyAdmin is an mysql administration frontend	https://charts.helm.sh/stable
%s/charts/bitnami/phpmyadmin	3.0.0        	4.9.0-1    	phpMyAdmin is an mysql administration frontend	https://charts.bitnami.com   
`, ts.URL, ts.URL)

	testcmd := "search hub --list-repo-url --endpoint " + ts.URL + " maria"
	storage := storageFixture()
	_, out, err := executeActionCommandC(storage, testcmd)
	if err != nil {
		t.Errorf("unexpected error, %s", err)
	}
	if out != expected {
		t.Error("expected and actual output did not match")
		t.Log(out)
		t.Log(expected)
	}
}

func TestSearchHubOutputCompletion(t *testing.T) {
	outputFlagCompletionTest(t, "search hub")
}

func TestSearchHubFileCompletion(t *testing.T) {
	checkFileCompletion(t, "search hub", true) // File completion may be useful when inputting a keyword
}

func TestSearchHubCmdExitCode(t *testing.T) {

	if os.Getenv("RUN_MAIN_FOR_TESTING") == "1" {
		// Setup a mock search service
		var searchResult = `{"data":[]}`
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, searchResult)
		}))
		defer ts.Close()

		os.Args = []string{"helm", "search", "hub", "syzygy", "--endpoint", ts.URL, "--fail-if-no-results-found"}

		// We DO call helm's main() here. So this looks like a normal `helm` process.
		main()

		// As main calls os.Exit, we never reach this line.
		// But the test called this block of code catches and verifies the exit code.
		return
	}

	// Do a second run of this specific test(TestPluginExitCode) with RUN_MAIN_FOR_TESTING=1 set,
	// So that the second run is able to run main() and this first run can verify the exit status returned by that.
	//
	// This technique originates from https://talks.golang.org/2014/testing.slide#23.
	cmd := exec.Command(os.Args[0], "-test.run=TestSearchHubCmdExitCode")
	cmd.Env = append(
		os.Environ(),
		"RUN_MAIN_FOR_TESTING=1",
	)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	exiterr, ok := err.(*exec.ExitError)

	if !ok {
		t.Fatalf("Unexpected error returned by os.Exit: %T", err)
	}

	expectedStdout := "No results found\n"
	if stdout.String() != expectedStdout {
		t.Errorf("Expected %q written to stdout: Got %q", expectedStdout, stdout.String())
	}

	if stderr.String() != "" {
		t.Errorf("Expected no writes to stderr: Got %q", stderr.String())
	}

	if exiterr.ExitCode() != 1 {
		t.Errorf("Expected exit code 1: Got %d", exiterr.ExitCode())
	}

}
