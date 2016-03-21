/*
Copyright 2015 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package repo

import (
	"fmt"
	"net/url"
)

// repo describes a repository
type repo struct {
	Name           string     `json:"name"`           // Friendly name for this repository
	URL            string     `json:"url"`            // URL to the root of this repository
	CredentialName string     `json:"credentialname"` // Credential name used to access this repository
	Format         RepoFormat `json:"format"`         // Format of this repository
	Type           RepoType   `json:"type"`           // Technology implementing this repository
}

func NewRepo(name, URL, credentialName, repoFormat, repoType string) (Repo, error) {
	return newRepo(name, URL, credentialName, RepoFormat(repoFormat), RepoType(repoType))
}

func newRepo(name, URL, credentialName string, repoFormat RepoFormat, repoType RepoType) (*repo, error) {
	if name == "" {
		return nil, fmt.Errorf("name must not be empty")
	}

	_, err := url.Parse(URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL (%s): %s", URL, err)
	}

	if credentialName == "" {
		credentialName = "default"
	}

	if err := validateRepoFormat(repoFormat); err != nil {
		return nil, err
	}

	r := &repo{
		Name:           name,
		Type:           repoType,
		URL:            URL,
		Format:         repoFormat,
		CredentialName: credentialName,
	}

	return r, nil
}

// Currently, only flat repositories are supported.
func validateRepoFormat(repoFormat RepoFormat) error {
	switch repoFormat {
	case FlatRepoFormat:
		return nil
	}

	return fmt.Errorf("unknown repository format: %s", repoFormat)
}

// GetName returns the friendly name of this repository.
func (r *repo) GetName() string {
	return r.Name
}

// GetType returns the technology implementing this repository.
func (r *repo) GetType() RepoType {
	return r.Type
}

// GetURL returns the URL to the root of this repository.
func (r *repo) GetURL() string {
	return r.URL
}

// GetFormat returns the format of this repository.
func (r *repo) GetFormat() RepoFormat {
	return r.Format
}

// GetCredentialName returns the credential name used to access this repository.
func (r *repo) GetCredentialName() string {
	return r.CredentialName
}

func validateRepo(tr Repo, wantName, wantURL, wantCredentialName string, wantFormat RepoFormat, wantType RepoType) error {
	haveName := tr.GetName()
	if haveName != wantName {
		return fmt.Errorf("unexpected repo name; want: %s, have %s", wantName, haveName)
	}

	haveURL := tr.GetURL()
	if haveURL != wantURL {
		return fmt.Errorf("unexpected repo url; want: %s, have %s", wantURL, haveURL)
	}

	haveCredentialName := tr.GetCredentialName()
	if wantCredentialName == "" {
		wantCredentialName = "default"
	}

	if haveCredentialName != wantCredentialName {
		return fmt.Errorf("unexpected repo credential name; want: %s, have %s", wantCredentialName, haveCredentialName)
	}

	haveFormat := tr.GetFormat()
	if haveFormat != wantFormat {
		return fmt.Errorf("unexpected repo format; want: %s, have %s", wantFormat, haveFormat)
	}

	haveType := tr.GetType()
	if haveType != wantType {
		return fmt.Errorf("unexpected repo type; want: %s, have %s", wantType, haveType)
	}

	return nil
}
