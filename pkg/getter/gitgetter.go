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

package getter

import (
	"bytes"
	"strings"

	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/vcs"

	"helm.sh/helm/v3/internal/fileutil"
)

// GitGetter is the default HTTP(/S) backend handler
type GitGetter struct {
	opts options
}

func (g *GitGetter) ChartName() string {
	return g.opts.chartName
}

// ensureGitDirIgnored will append ".git/" to the .helmignore file in a directory.
// Create the .helmignore file if it does not exist.
func (g *GitGetter) ensureGitDirIgnored(repoPath string) error {
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

//Get performs a Get from repo.Getter and returns the body.
func (g *GitGetter) Get(href string, options ...Option) (*bytes.Buffer, error) {
	for _, opt := range options {
		opt(&g.opts)
	}
	return g.get(href)
}

func (g *GitGetter) get(href string) (*bytes.Buffer, error) {
	gitURL := strings.TrimPrefix(href, "git://")
	version := g.opts.version
	chartName := g.opts.chartName
	if version == "" {
		return nil, fmt.Errorf("the version must be a valid tag or branch name for the git repo, not nil")
	}
	tmpDir, err := os.MkdirTemp("", "helm")
	if err != nil {
		return nil, err
	}
	chartTmpDir := filepath.Join(tmpDir, chartName)

	if err := os.MkdirAll(chartTmpDir, 0755); err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	repo, err := vcs.NewRepo(gitURL, chartTmpDir)
	if err != nil {
		return nil, err
	}
	if err := repo.Get(); err != nil {
		return nil, err
	}
	if err := repo.UpdateVersion(version); err != nil {
		return nil, err
	}

	// A .helmignore that includes an ignore for .git/ should be included in the git repo itself,
	// but a lot of people will probably not think about that.
	// To prevent the git history from bleeding into the charts archive, append/create .helmignore.
	g.ensureGitDirIgnored(chartTmpDir)

	buf, err := fileutil.CompressDirToTgz(chartTmpDir, tmpDir)
	if err != nil {
		return nil, fmt.Errorf("unable to tar and compress dir %s to tgz file. %s", tmpDir, err)
	}
	return buf, nil
}

// NewGitGetter constructs a valid git client as a Getter
func NewGitGetter(ops ...Option) (Getter, error) {

	client := GitGetter{}

	for _, opt := range ops {
		opt(&client.opts)
	}

	return &client, nil
}
