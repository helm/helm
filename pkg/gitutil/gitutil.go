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
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var execCommand = exec.Command

// This regex is designed to match output from git of the style:
//   ebeb6eafceb61dd08441ffe086c77eb472842494  refs/tags/v0.21.0
// and extract the hash and ref name as capture groups
var gitRefLineRegexp = regexp.MustCompile(`^([a-fA-F0-9]+)\s+(refs\/(?:tags|heads|pull|remotes)\/)(.*)$`)

// Run a git command as a child process.
// If git is not on the path, an error will be returned.
// Returns the command output or an error.
func gitExec(args ...string) ([]byte, error) {
	cmd := execCommand("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("Error executing git command:\ngit %s\n\n%s\n%s", strings.Join(args, " "), string(output), err)
	}
	return output, err
}

// GetRefs loads the tags, refs, branches (commit-ish) from a git repo.
// Returns a map of tags and branch names to commit shas
func GetRefs(gitRepoURL string) (map[string]string, error) {
	output, err := gitExec("ls-remote", "--tags", "--heads", gitRepoURL)
	if err != nil {
		return nil, err
	}

	tagsToCommitShas := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Bytes()
		match := gitRefLineRegexp.FindSubmatch(line)
		if len(match) == 4 {
			// As documented in gitrevisions: https://www.kernel.org/pub/software/scm/git/docs/gitrevisions.html#_specifying_revisions
			// "A suffix ^ followed by an empty brace pair means the object could be a tag, and dereference the tag recursively until a non-tag object is found."
			// In other words, the hash without ^{} is the hash of the tag, and the hash with ^{} is the hash of the commit at which the tag was made.
			// For our purposes, either will work.
			var name = strings.TrimSuffix(string(match[3]), "^{}")
			tagsToCommitShas[name] = string(match[1])
		}
	}

	return tagsToCommitShas, nil
}

// CloneTo fetches a git repo at a specific ref from a git url
func CloneTo(gitRepoURL string, ref string, destinationPath string) error {
	_, err := gitExec("clone", "--depth", "1", gitRepoURL, "--branch", ref, "--single-branch", destinationPath)
	return err
}
