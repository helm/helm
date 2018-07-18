/*
Copyright The Helm maintainers

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

package notify // import "k8s.io/helm/pkg/helm/notify"

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"k8s.io/helm/pkg/version"
)

var (
	// The time format used in the file recording the last check
	timeLayout = time.RFC1123Z
)

// Release contains information for a release
type Release struct {

	// Version is the version for the release (e.g., `v2.11.0`)
	Version string `json:"version"`

	// Checksums is a map of the file checksums for a release. For example,
	// a key might be "darwin-amd64" and the value is the checksum for the release
	// in that environment
	Checksums map[string]string `json:"checksums,omitempty"`
}

// Releases is an array of Release
type Releases []Release

// IfTime checks if there is a newer version of Helm if the wait period is over
// Arguments are:
// - lastUpdateTime: The last time an update was checked
// - waitTime: The period to wait before checking again
// - checkURL: A URL with a JSON file containing the updates
// TODO: Change checkURL to the internal object
// The return data includes:
// - bool: true if an was checked for and false otherwise
// - string: the latest version if there is a newer version and an empty string otherwise
func IfTime(lastUpdateTime time.Time, waitTime time.Duration, checkURL string) (bool, string, error) {
	// Check if current time has waited long enough
	curr := time.Since(lastUpdateTime)
	if curr <= waitTime {
		return false, "", nil
	}

	// Check for an update
	releases, err := getJSON(checkURL)
	if err != nil {
		return true, "", err
	}

	// Check if there is a new release
	curSemVer, err := semver.NewVersion(version.GetVersion())
	if err != nil {
		return true, "", err
	}

	var vs []*semver.Version

	for _, release := range releases {
		tmpVer, err := semver.NewVersion(release.Version)
		if err == nil {
			vs = append(vs, tmpVer)
		}
	}
	sort.Sort(semver.Collection(vs))

	if len(vs) > 0 && vs[len(vs)-1].GreaterThan(curSemVer) {
		return true, vs[len(vs)-1].String(), nil
	}

	// Return new release if one found
	return true, "", nil
}

// IfTimeFromFile will notify of a new version available if:
// - The current time is greater than waitTime + the last time it was updated
// - There is a newer version available
// This is a helper function wrapping IfTime, reading from the filesystem,
// and pretty printing if there is a new version.
// The arguments are:
// - lastUpdatePath: The path to the local filesystem location containing the
//   last time an update was checked for
// - waitTime: The time in seconds to wait from the last check before checking
//   again.
// - checkURL: A URL to a JSON file with version information to check for updates
func IfTimeFromFile(lastUpdatePath string, waitTime int64, checkURL string) (string, error) {
	// Read contents of the time file
	content, err := ioutil.ReadFile(lastUpdatePath)

	if err != nil {
		// The file does not exist so we will assume this is a first run and create
		// the file with a time of now so it will tell about an update in the future
		errStr := err.Error()
		if strings.Contains(errStr, "no such file or directory") {
			curTime := time.Now().Format(timeLayout)
			ioutil.WriteFile(lastUpdatePath, []byte(curTime), 0644)
			content = []byte(curTime)
		} else {
			return "", err
		}
	}

	// Convert the string time to a usable Time instance
	lastChecked, err := time.Parse(timeLayout, strings.TrimSpace(string(content)))
	if err != nil {
		return "", err
	}

	// Convert the wait period to a Duration instance
	waitPeriod := time.Duration(waitTime) * time.Second

	// Call NotifyIfTime
	checked, newVersion, err := IfTime(lastChecked, waitPeriod, checkURL)

	if err != nil {
		return "", err
	}

	// Update the check time if a check happened
	if checked {
		curTime := time.Now().Format(timeLayout)
		ioutil.WriteFile(lastUpdatePath, []byte(curTime), 0644)

		// If there is a new version and a check happened print the message
		return newVersion, nil
	}

	return "", nil

}

func getJSON(href string) (Releases, error) {
	client := &http.Client{}

	// Construct the request
	req, err := http.NewRequest("GET", href, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Helm/"+strings.TrimPrefix(version.GetVersion(), "v"))

	// Perform the request and handle errors
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Failed to fetch update information at %s : %s", href, resp.Status)
	}

	// Get the contents of the JSON file
	target := Releases{}
	err = json.NewDecoder(resp.Body).Decode(&target)
	if err != nil {
		return nil, err
	}

	return target, nil
}
