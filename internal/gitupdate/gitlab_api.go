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

package gitupdate

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	"golang.org/x/oauth2"
)

func updateViaGitLabAPI(ctx context.Context, opts RepositoryOptions, result Result) (Result, error) {
	baseURL, project, err := resolveGitLabCoordinates(opts.RepositoryURL, opts.API)
	if err != nil {
		return result, err
	}
	client, err := newGitLabClient(opts, baseURL)
	if err != nil {
		return result, err
	}

	readBranch := opts.Branch
	if opts.CreateBranch {
		if err := requireGitLabBranch(ctx, client, project, opts.Branch, false); err != nil {
			return result, err
		}
		if err := requireGitLabBranch(ctx, client, project, opts.BaseBranch, true); err != nil {
			return result, err
		}
		readBranch = opts.BaseBranch
	} else if err := requireGitLabBranch(ctx, client, project, opts.Branch, true); err != nil {
		return result, err
	}

	ref := readBranch
	file, response, getErr := client.RepositoryFiles.GetFile(
		project,
		result.TargetFile,
		&gitlab.GetFileOptions{Ref: &ref},
		gitlab.WithContext(ctx),
	)

	existed := true
	var original []byte
	var lastCommitID string
	switch {
	case getErr == nil:
		original, err = decodeGitLabFile(file)
		if err != nil {
			return result, fmt.Errorf("decode GitLab file %q: %w", result.TargetFile, err)
		}
		lastCommitID = file.LastCommitID
	case isGitLabStatus(response, http.StatusNotFound):
		if !opts.CreateFile {
			return result, fmt.Errorf("target file %q does not exist on branch %q (use --create-file to create it)", result.TargetFile, readBranch)
		}
		existed = false
		original = initialContent(opts.Edit.Method)
	default:
		return result, fmt.Errorf("read target file %q through GitLab API: %w", result.TargetFile, getErr)
	}

	updated, err := Apply(original, opts.Edit)
	if err != nil {
		return result, fmt.Errorf("edit %q: %w", result.TargetFile, err)
	}
	result.Changed = !existed || !bytes.Equal(original, updated)
	if !result.Changed || opts.DryRun {
		return result, nil
	}

	action := gitlab.FileUpdate
	if !existed {
		action = gitlab.FileCreate
	}
	encoding := "base64"
	content := base64.StdEncoding.EncodeToString(updated)
	commitOptions := &gitlab.CreateCommitOptions{
		Branch:        &opts.Branch,
		CommitMessage: &opts.CommitMessage,
		AuthorName:    &opts.AuthorName,
		AuthorEmail:   &opts.AuthorEmail,
		Actions: []*gitlab.CommitActionOptions{{
			Action:   &action,
			FilePath: &result.TargetFile,
			Content:  &content,
			Encoding: &encoding,
		}},
	}
	if existed {
		commitOptions.Actions[0].LastCommitID = &lastCommitID
	}
	if opts.CreateBranch {
		commitOptions.StartBranch = &opts.BaseBranch
	}

	commit, _, err := client.Commits.CreateCommit(project, commitOptions, gitlab.WithContext(ctx))
	if err != nil {
		return result, fmt.Errorf("commit target file %q through GitLab API: %w", result.TargetFile, err)
	}
	result.Committed = true
	result.Pushed = true
	result.CommitHash = commit.ID
	return result, nil
}

func requireGitLabBranch(ctx context.Context, client *gitlab.Client, project, branch string, mustExist bool) error {
	_, response, err := client.Branches.GetBranch(project, branch, gitlab.WithContext(ctx))
	exists := err == nil
	if err != nil && !isGitLabStatus(response, http.StatusNotFound) {
		return fmt.Errorf("check GitLab branch %q: %w", branch, err)
	}
	if mustExist && !exists {
		return fmt.Errorf("GitLab branch %q does not exist", branch)
	}
	if !mustExist && exists {
		return fmt.Errorf("cannot create branch %q: it already exists on GitLab", branch)
	}
	return nil
}

func newGitLabClient(opts RepositoryOptions, baseURL string) (*gitlab.Client, error) {
	httpClient, err := opts.TLS.httpClient()
	if err != nil {
		return nil, err
	}
	clientOptions := []gitlab.ClientOptionFunc{
		gitlab.WithBaseURL(baseURL),
		gitlab.WithHTTPClient(httpClient),
		gitlab.WithOnlyIdempotentRetries(),
	}

	tokenType, err := resolveAPITokenType(opts.API.TokenType, opts.Auth.Type)
	if err != nil {
		return nil, err
	}
	switch tokenType {
	case APITokenPrivate:
		return gitlab.NewClient(opts.Auth.Token, clientOptions...)
	case APITokenOAuth:
		source := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: opts.Auth.Token})
		return gitlab.NewAuthSourceClient(gitlab.OAuthTokenSource{TokenSource: source}, clientOptions...)
	case APITokenJob:
		return gitlab.NewJobClient(opts.Auth.Token, clientOptions...)
	default:
		return nil, fmt.Errorf("unsupported GitLab API token type %q", tokenType)
	}
}

func resolveGitLabCoordinates(repositoryURL string, apiOpts APIOptions) (string, string, error) {
	return resolveAPICoordinates(repositoryURL, apiOpts, "/api/v4/", "GitLab")
}

func inferGitLabCoordinates(repositoryURL string) (string, string, error) {
	host, project, scheme, err := inferRepositoryCoordinates(repositoryURL)
	if err != nil {
		return "", "", err
	}
	return fmt.Sprintf("%s://%s/api/v4/", scheme, host), project, nil
}

func decodeGitLabFile(file *gitlab.File) ([]byte, error) {
	switch file.Encoding {
	case "", "base64":
		return base64.StdEncoding.DecodeString(file.Content)
	case "text":
		return []byte(file.Content), nil
	default:
		return nil, fmt.Errorf("unsupported content encoding %q", file.Encoding)
	}
}

func isGitLabStatus(response *gitlab.Response, status int) bool {
	return response != nil && response.StatusCode == status
}

func (opts TLSOptions) httpClient() (*http.Client, error) {
	loaded, err := opts.load()
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: loaded.insecureSkipTLS, //nolint:gosec
	}
	if len(loaded.caBundle) != 0 {
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(loaded.caBundle) {
			return nil, errors.New("git CA file does not contain a valid certificate")
		}
		tlsConfig.RootCAs = roots
	}
	if len(loaded.clientCertificate) != 0 {
		certificate, err := tls.X509KeyPair(loaded.clientCertificate, loaded.clientKey)
		if err != nil {
			return nil, fmt.Errorf("load Git client certificate and key: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}
	transport.TLSClientConfig = tlsConfig
	return &http.Client{Transport: transport}, nil
}
