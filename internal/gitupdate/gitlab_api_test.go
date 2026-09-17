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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateViaGitLabAPI(t *testing.T) {
	original := "image:\n  tag: v1\n"
	var commitRequest gitLabCommitRequest
	requests := []string{}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.EscapedPath())
		assert.Equal(t, "test-token", request.Header.Get("PRIVATE-TOKEN"))
		writer.Header().Set("Content-Type", "application/json")

		switch len(requests) {
		case 1:
			assert.Contains(t, request.URL.EscapedPath(), "/repository/branches/main")
			fmt.Fprint(writer, `{"name":"main","can_push":true}`)
		case 2:
			assert.Contains(t, request.URL.EscapedPath(), "/repository/files/")
			assert.Equal(t, "main", request.URL.Query().Get("ref"))
			writeGitLabFileResponse(t, writer, "values.yaml", original, "file-commit-1")
		case 3:
			assert.Equal(t, http.MethodPost, request.Method)
			assert.Contains(t, request.URL.EscapedPath(), "/repository/commits")
			if !assert.NoError(t, json.NewDecoder(request.Body).Decode(&commitRequest)) {
				http.Error(writer, "invalid request body", http.StatusBadRequest)
				return
			}
			fmt.Fprint(writer, `{"id":"0123456789abcdef","short_id":"01234567"}`)
		default:
			t.Errorf("unexpected request: %s %s", request.Method, request.URL)
			http.Error(writer, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	options := apiRepositoryFixture(server.URL)
	result, err := Update(context.Background(), options)
	require.NoError(t, err)
	assert.True(t, result.Changed)
	assert.True(t, result.Committed)
	assert.True(t, result.Pushed)
	assert.Equal(t, TransportAPI, result.Transport)
	assert.Equal(t, "0123456789abcdef", result.CommitHash)
	require.Len(t, commitRequest.Actions, 1)
	assert.Equal(t, "main", commitRequest.Branch)
	assert.Equal(t, "update", commitRequest.Actions[0].Action)
	assert.Equal(t, "values.yaml", commitRequest.Actions[0].FilePath)
	assert.Equal(t, "file-commit-1", commitRequest.Actions[0].LastCommitID)
	assert.Equal(t, "base64", commitRequest.Actions[0].Encoding)
	updated, err := base64.StdEncoding.DecodeString(commitRequest.Actions[0].Content)
	require.NoError(t, err)
	assert.Equal(t, "image:\n  tag: v2\n", string(updated))
}

func TestUpdateViaGitLabAPIDryRunDoesNotCommit(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		writer.Header().Set("Content-Type", "application/json")
		switch requestCount {
		case 1:
			fmt.Fprint(writer, `{"name":"main","can_push":true}`)
		case 2:
			writeGitLabFileResponse(t, writer, "values.yaml", "image:\n  tag: v1\n", "file-commit-1")
		default:
			t.Errorf("dry run made unexpected request: %s %s", request.Method, request.URL)
			http.Error(writer, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	options := apiRepositoryFixture(server.URL)
	options.DryRun = true
	result, err := Update(context.Background(), options)
	require.NoError(t, err)
	assert.True(t, result.Changed)
	assert.False(t, result.Committed)
	assert.False(t, result.Pushed)
	assert.Equal(t, 2, requestCount)
}

func TestUpdateViaGitLabAPICreatesBranchAndFile(t *testing.T) {
	requestCount := 0
	var commitRequest gitLabCommitRequest
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		writer.Header().Set("Content-Type", "application/json")
		switch requestCount {
		case 1:
			http.Error(writer, `{"message":"404 Branch Not Found"}`, http.StatusNotFound)
		case 2:
			fmt.Fprint(writer, `{"name":"main","can_push":true}`)
		case 3:
			assert.Equal(t, "main", request.URL.Query().Get("ref"))
			http.Error(writer, `{"message":"404 File Not Found"}`, http.StatusNotFound)
		case 4:
			if !assert.NoError(t, json.NewDecoder(request.Body).Decode(&commitRequest)) {
				http.Error(writer, "invalid request body", http.StatusBadRequest)
				return
			}
			fmt.Fprint(writer, `{"id":"new-branch-commit"}`)
		default:
			t.Errorf("unexpected request: %s %s", request.Method, request.URL)
		}
	}))
	defer server.Close()

	options := apiRepositoryFixture(server.URL)
	options.Branch = "feature/demo"
	options.BaseBranch = "main"
	options.CreateBranch = true
	options.TargetFile = "config/new.yaml"
	options.CreateFile = true
	options.Edit = EditOptions{
		Method:          MethodYAMLSet,
		Content:         []byte("true"),
		ContentProvided: true,
		Pointer:         "/enabled",
	}

	result, err := Update(context.Background(), options)
	require.NoError(t, err)
	assert.True(t, result.Pushed)
	assert.Equal(t, "feature/demo", commitRequest.Branch)
	assert.Equal(t, "main", commitRequest.StartBranch)
	require.Len(t, commitRequest.Actions, 1)
	assert.Equal(t, "create", commitRequest.Actions[0].Action)
	assert.Empty(t, commitRequest.Actions[0].LastCommitID)
	updated, err := base64.StdEncoding.DecodeString(commitRequest.Actions[0].Content)
	require.NoError(t, err)
	assert.Equal(t, "{enabled: true}\n", string(updated))
}

func TestInferGitLabCoordinates(t *testing.T) {
	tests := []struct {
		name        string
		repository  string
		expectedURL string
		project     string
	}{
		{
			name:        "HTTPS",
			repository:  "https://gitlab.example.com/acme/charts.git",
			expectedURL: "https://gitlab.example.com/api/v4/",
			project:     "acme/charts",
		},
		{
			name:        "SSH URL",
			repository:  "ssh://git@gitlab.example.com/acme/charts.git",
			expectedURL: "https://gitlab.example.com/api/v4/",
			project:     "acme/charts",
		},
		{
			name:        "SCP syntax",
			repository:  "git@gitlab.example.com:acme/charts.git",
			expectedURL: "https://gitlab.example.com/api/v4/",
			project:     "acme/charts",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			baseURL, project, err := inferGitLabCoordinates(test.repository)
			require.NoError(t, err)
			assert.Equal(t, test.expectedURL, baseURL)
			assert.Equal(t, test.project, project)
		})
	}
}

func TestValidateAPIOptions(t *testing.T) {
	options := apiRepositoryFixture("https://gitlab.example.com")

	tests := []struct {
		name      string
		mutate    func(*RepositoryOptions)
		errorText string
	}{
		{
			name: "missing token",
			mutate: func(options *RepositoryOptions) {
				options.Auth.Token = ""
			},
			errorText: "requires --token",
		},
		{
			name: "SSH auth is rejected",
			mutate: func(options *RepositoryOptions) {
				options.Auth.Type = AuthSSHKey
			},
			errorText: "not supported by the gitlab API transport",
		},
		{
			name: "push false is rejected",
			mutate: func(options *RepositoryOptions) {
				options.Push = false
			},
			errorText: "requires --push=true",
		},
		{
			name: "unknown provider",
			mutate: func(options *RepositoryOptions) {
				options.API.Provider = "github"
			},
			errorText: "unknown API provider",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := options
			test.mutate(&candidate)
			err := validateRepositoryOptions(candidate)
			require.ErrorContains(t, err, test.errorText)
		})
	}
}

func apiRepositoryFixture(serverURL string) RepositoryOptions {
	return RepositoryOptions{
		RepositoryURL: serverURL + "/group/repo.git",
		Transport:     TransportAPI,
		Branch:        "main",
		TargetFile:    "values.yaml",
		Edit: EditOptions{
			Method:          MethodYAMLSet,
			Content:         []byte("v2"),
			ContentProvided: true,
			Pointer:         "/image/tag",
		},
		Auth: AuthOptions{
			Type:  AuthToken,
			Token: "test-token",
		},
		API: APIOptions{
			Provider:  APIProviderGitLab,
			BaseURL:   serverURL + "/api/v4/",
			Project:   "group/repo",
			TokenType: APITokenPrivate,
		},
		AuthorName:    "Automation",
		AuthorEmail:   "automation@example.com",
		CommitMessage: "update through API",
		Depth:         1,
		Push:          true,
	}
}

func writeGitLabFileResponse(t *testing.T, writer http.ResponseWriter, path, content, lastCommitID string) {
	t.Helper()
	response := map[string]any{
		"file_name":      path[strings.LastIndex(path, "/")+1:],
		"file_path":      path,
		"encoding":       "base64",
		"content":        base64.StdEncoding.EncodeToString([]byte(content)),
		"last_commit_id": lastCommitID,
	}
	require.NoError(t, json.NewEncoder(writer).Encode(response))
}

type gitLabCommitRequest struct {
	Branch        string                      `json:"branch"`
	StartBranch   string                      `json:"start_branch"`
	CommitMessage string                      `json:"commit_message"`
	AuthorName    string                      `json:"author_name"`
	AuthorEmail   string                      `json:"author_email"`
	Actions       []gitLabCommitActionRequest `json:"actions"`
}

type gitLabCommitActionRequest struct {
	Action       string `json:"action"`
	FilePath     string `json:"file_path"`
	Content      string `json:"content"`
	Encoding     string `json:"encoding"`
	LastCommitID string `json:"last_commit_id"`
}
