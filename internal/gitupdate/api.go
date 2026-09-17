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
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
)

const (
	APIProviderAuto   = "auto"
	APIProviderGitLab = "gitlab"

	APITokenAuto    = "auto"
	APITokenPrivate = "private"
	APITokenOAuth   = "oauth"
	APITokenJob     = "job"
)

// APIOptions configures direct repository-provider API access.
type APIOptions struct {
	Provider  string
	BaseURL   string
	Project   string
	TokenType string
}

// ValidAPIProviders returns the repository APIs supported by TransportAPI.
func ValidAPIProviders() []string {
	return []string{APIProviderAuto, APIProviderGitLab}
}

// ValidAPITokenTypes returns the authentication token types supported by API
// backends. Individual providers may support only a subset.
func ValidAPITokenTypes() []string {
	return []string{APITokenAuto, APITokenPrivate, APITokenOAuth, APITokenJob}
}

func validateAPIOptions(opts RepositoryOptions) error {
	provider, err := resolveAPIProvider(opts.API)
	if err != nil {
		return err
	}
	if _, err := resolveAPITokenType(opts.API.TokenType, opts.Auth.Type); err != nil {
		return err
	}
	if opts.Auth.Token == "" {
		return errors.New("API transport requires --token, --token-file, or HELM_GIT_TOKEN")
	}
	switch opts.Auth.Type {
	case "", AuthAuto, AuthToken, AuthBearer:
	default:
		return fmt.Errorf("authentication mode %q is not supported by the %s API transport", opts.Auth.Type, provider)
	}
	if opts.Auth.Password != "" || opts.Auth.SSHPrivateKey != "" || opts.Auth.SSHKeyPassphrase != "" ||
		opts.Auth.KnownHostsFile != "" || opts.Auth.InsecureHostKey {
		return fmt.Errorf("password and SSH authentication options are not supported by the %s API transport", provider)
	}
	if !opts.Push && !opts.DryRun {
		return errors.New("API transport writes commits remotely and requires --push=true (use --dry-run to validate without writing)")
	}

	_, _, err = resolveGitLabCoordinates(opts.RepositoryURL, opts.API)
	return err
}

func updateViaAPI(ctx context.Context, opts RepositoryOptions, result Result) (Result, error) {
	if _, err := resolveAPIProvider(opts.API); err != nil {
		return result, err
	}
	return updateViaGitLabAPI(ctx, opts, result)
}

func resolveAPIProvider(apiOpts APIOptions) (string, error) {
	provider := apiOpts.Provider
	if provider == "" {
		provider = APIProviderAuto
	}
	if provider == APIProviderAuto || provider == APIProviderGitLab {
		return APIProviderGitLab, nil
	}
	return "", fmt.Errorf("unknown API provider %q (supported: %s)", provider, strings.Join(ValidAPIProviders(), ", "))
}

func resolveAPITokenType(tokenType, authType string) (string, error) {
	if tokenType == "" || tokenType == APITokenAuto {
		if authType == AuthBearer {
			return APITokenOAuth, nil
		}
		return APITokenPrivate, nil
	}
	if slices.Contains(ValidAPITokenTypes(), tokenType) {
		return tokenType, nil
	}
	return "", fmt.Errorf("unknown API token type %q (supported: %s)", tokenType, strings.Join(ValidAPITokenTypes(), ", "))
}

func resolveAPICoordinates(repositoryURL string, apiOpts APIOptions, apiPath, providerName string) (string, string, error) {
	inferredHost, inferredProject, inferredScheme, err := inferRepositoryCoordinates(repositoryURL)
	if err != nil && (apiOpts.BaseURL == "" || apiOpts.Project == "") {
		return "", "", err
	}

	baseURL := apiOpts.BaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("%s://%s/%s", inferredScheme, inferredHost, strings.Trim(apiPath, "/"))
	}
	project := apiOpts.Project
	if project == "" {
		project = inferredProject
	}
	if project == "" {
		return "", "", fmt.Errorf("%s API project is empty (set --api-project)", providerName)
	}

	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", "", fmt.Errorf("invalid %s API base URL %q", providerName, baseURL)
	}
	return strings.TrimRight(baseURL, "/") + "/", project, nil
}

func inferRepositoryCoordinates(repositoryURL string) (host, project, scheme string, err error) {
	if strings.Contains(repositoryURL, "://") {
		parsed, parseErr := url.Parse(repositoryURL)
		if parseErr != nil {
			return "", "", "", fmt.Errorf("parse repository URL for API access: %w", parseErr)
		}
		host = parsed.Host
		project = strings.Trim(parsed.Path, "/")
		switch parsed.Scheme {
		case "http", "https":
			scheme = parsed.Scheme
		case "ssh", "git+ssh":
			scheme = "https"
		default:
			return "", "", "", fmt.Errorf("cannot infer API URL from repository scheme %q", parsed.Scheme)
		}
	} else {
		at := strings.IndexByte(repositoryURL, '@')
		colon := strings.IndexByte(repositoryURL, ':')
		if at <= 0 || colon <= at+1 {
			return "", "", "", errors.New("cannot infer API coordinates from repository URL (set --api-base-url, --api-project, and --api-provider)")
		}
		scheme = "https"
		host = repositoryURL[at+1 : colon]
		project = repositoryURL[colon+1:]
	}

	project = strings.TrimSuffix(strings.Trim(project, "/"), ".git")
	if host == "" || project == "" {
		return "", "", "", errors.New("repository URL does not contain a host and project path")
	}
	return host, project, scheme, nil
}
