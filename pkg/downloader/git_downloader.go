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

package downloader

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/gitutil"
)

// Assigned here so it can be overridden for testing.
var gitCloneTo = gitutil.CloneTo

// GitDownloader handles downloading a chart from a git url.
type GitDownloader struct{}

// ensureGitDirIgnored will append ".git/" to the .helmignore file in a directory.
// Create the .helmignore file if it does not exist.
func (g *GitDownloader) ensureGitDirIgnored(repoPath string) error {
	helmignorePath := filepath.Join(repoPath, ".helmignore")
	f, err := os.OpenFile(helmignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString("\n.git/\n"); err != nil {
		return err
	}
	return nil
}

// DownloadTo will create a temp directory, then fetch a git repo into it.
// The git repo will be archived into a chart and copied to the destPath.
func (g *GitDownloader) DownloadTo(gitURL string, ref string, destPath string) error {
	// the git archive command returns a tgz archive. we need to extract it to get the actual chart files.
	tmpDir, err := ioutil.TempDir("", "helm")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if err = gitCloneTo(gitURL, ref, tmpDir); err != nil {
		return fmt.Errorf("Unable to retrieve git repo. %s", err)
	}

	// A .helmignore that includes an ignore for .git/ should be included in the git repo itself,
	// but a lot of people will probably not think about that.
	// To prevent the git history from bleeding into the charts archive, append/create .helmignore.
	g.ensureGitDirIgnored(tmpDir)

	// Turn the extracted git archive into a chart and move it into the charts directory.
	// This is using chartutil.Save() so that .helmignore logic is applied.
	loadedChart, loadErr := loader.LoadDir(tmpDir)
	if loadErr != nil {
		return fmt.Errorf("Unable to process the git repo %s as a chart. %s", gitURL, err)
	}
	if _, saveErr := chartutil.Save(loadedChart, destPath); saveErr != nil {
		return fmt.Errorf("Unable to save the git repo %s as a chart. %s", gitURL, err)
	}
	return nil
}
