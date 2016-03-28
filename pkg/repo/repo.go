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
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// URL is the url pattern used to check if a given repo url is valid
const URL string = `^((http|gs|https?):\/\/)?(\S+(:\S*)?@)?((([1-9]\d?|1\d\d|2[01]\d|22[0-3])(\.(1?\d{1,2}|2[0-4]\d|25[0-5])){2}(?:\.([0-9]\d?|1\d\d|2[0-4]\d|25[0-4]))|(([a-zA-Z0-9]+([-\.][a-zA-Z0-9]+)*)|((www\.)?))?(([a-zA-Z\x{00a1}-\x{ffff}0-9]+-?-?)*[a-zA-Z\x{00a1}-\x{ffff}0-9]+)(?:\.([a-zA-Z\x{00a1}-\x{ffff}]{2,}))?))(:(\d{1,5}))?((\/|\?|#)[^\s]*)?$`

// NewRepo takes params and returns a IRepo
func NewRepo(URL, credentialName, repoName, repoFormat, repoType string) (IRepo, error) {
	return newRepo(URL, credentialName, repoName, ERepoFormat(repoFormat), ERepoType(repoType))
}

func newRepo(URL, credentialName, repoName string, repoFormat ERepoFormat, repoType ERepoType) (*Repo, error) {
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

	r := &Repo{
		Name:           repoName,
		URL:            URL,
		CredentialName: credentialName,
		Type:           repoType,
		Format:         repoFormat,
	}

	return r, nil
}

// Currently, only flat repositories are supported.
func validateRepoFormat(repoFormat ERepoFormat) error {
	switch repoFormat {
	case FlatRepoFormat:
		return nil
	}

	return fmt.Errorf("unknown repository format: %s", repoFormat)
}

// GetType returns the technology implementing this repository.
func (r *Repo) GetType() ERepoType {
	return r.Type
}

// GetName returns the name of this repository.
func (r *Repo) GetName() string {
	return r.Name
}

// GetURL returns the URL to the root of this repository.
func (r *Repo) GetURL() string {
	return r.URL
}

// GetFormat returns the format of this repository.
func (r *Repo) GetFormat() ERepoFormat {
	return r.Format
}

// GetCredentialName returns the credential name used to access this repository.
func (r *Repo) GetCredentialName() string {
	return r.CredentialName
}

func validateRepo(tr IRepo, wantURL, wantCredentialName string, wantFormat ERepoFormat, wantType ERepoType) error {
	haveURL := tr.GetURL()
	if haveURL != wantURL {
		return fmt.Errorf("unexpected repository url; want: %s, have %s", wantURL, haveURL)
	}

	haveCredentialName := tr.GetCredentialName()
	if wantCredentialName == "" {
		wantCredentialName = "default"
	}

	if haveCredentialName != wantCredentialName {
		return fmt.Errorf("unexpected repository credential name; want: %s, have %s", wantCredentialName, haveCredentialName)
	}

	haveFormat := tr.GetFormat()
	if haveFormat != wantFormat {
		return fmt.Errorf("unexpected repository format; want: %s, have %s", wantFormat, haveFormat)
	}

	haveType := tr.GetType()
	if haveType != wantType {
		return fmt.Errorf("unexpected repository type; want: %s, have %s", wantType, haveType)
	}

	return nil
}

// ValidateRepoURL checks if the string is a valid URL.
// This was inspired by the IsURL function in govalidator https://github.com/asaskevich/govalidator
func ValidateRepoURL(str string) error {
	err := errors.New("Invalid Repository URL")
	if str == "" || len(str) >= 2083 || len(str) <= 3 || strings.HasPrefix(str, ".") {
		return err
	}
	u, err := url.Parse(str)
	if err != nil {
		return err
	}
	if strings.HasPrefix(u.Host, ".") {
		return err
	}
	if u.Host == "" && (u.Path != "" && !strings.Contains(u.Path, ".")) {
		return err
	}
	if !regexp.MustCompile(URL).MatchString(str) {
		return err
	}
	return nil
}
