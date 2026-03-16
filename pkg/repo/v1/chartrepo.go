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

package repo // import "helm.sh/helm/v4/pkg/repo/v1"

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"helm.sh/helm/v4/internal/fileutil"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/helmpath"
)

// Entry represents a collection of parameters for chart repository
type Entry struct {
	Name                  string `json:"name"`
	URL                   string `json:"url"`
	Username              string `json:"username"`
	Password              string `json:"password"`
	CertFile              string `json:"certFile"`
	KeyFile               string `json:"keyFile"`
	CAFile                string `json:"caFile"`
	InsecureSkipTLSVerify bool   `json:"insecure_skip_tls_verify"`
	PassCredentialsAll    bool   `json:"pass_credentials_all"`
}

// ChartRepository represents a chart repository
type ChartRepository struct {
	Config    *Entry
	IndexFile *IndexFile
	Client    getter.Getter
	CachePath string
}

// NewChartRepository constructs ChartRepository
func NewChartRepository(cfg *Entry, getters getter.Providers) (*ChartRepository, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid chart URL format: %s", cfg.URL)
	}

	client, err := getters.ByScheme(u.Scheme)
	if err != nil {
		return nil, fmt.Errorf("could not find protocol handler for: %s", u.Scheme)
	}

	return &ChartRepository{
		Config:    cfg,
		IndexFile: NewIndexFile(),
		Client:    client,
		CachePath: helmpath.CachePath("repository"),
	}, nil
}

// DownloadIndexFile fetches the index from a repository.
func (r *ChartRepository) DownloadIndexFile() (string, error) {
	indexURL, err := ResolveReferenceURL(r.Config.URL, "index.yaml")
	if err != nil {
		return "", err
	}

	resp, err := r.Client.Get(indexURL,
		getter.WithURL(r.Config.URL),
		getter.WithInsecureSkipVerifyTLS(r.Config.InsecureSkipTLSVerify),
		getter.WithTLSClientConfig(r.Config.CertFile, r.Config.KeyFile, r.Config.CAFile),
		getter.WithBasicAuth(r.Config.Username, r.Config.Password),
		getter.WithPassCredentialsAll(r.Config.PassCredentialsAll),
	)
	if err != nil {
		return "", err
	}

	index, err := io.ReadAll(resp)
	if err != nil {
		return "", err
	}

	indexFile, err := loadIndex(index, r.Config.URL)
	if err != nil {
		return "", err
	}

	// Create the chart list file in the cache directory
	var charts strings.Builder
	for name := range indexFile.Entries {
		fmt.Fprintln(&charts, name)
	}
	chartsFile := filepath.Join(r.CachePath, helmpath.CacheChartsFile(r.Config.Name))
	os.MkdirAll(filepath.Dir(chartsFile), 0755)

	fileutil.AtomicWriteFile(chartsFile, bytes.NewReader([]byte(charts.String())), 0644)

	// Create the index file in the cache directory
	fname := filepath.Join(r.CachePath, helmpath.CacheIndexFile(r.Config.Name))
	os.MkdirAll(filepath.Dir(fname), 0755)
	return fname, fileutil.AtomicWriteFile(fname, bytes.NewReader(index), 0644)
}

type findChartInRepoURLOptions struct {
	Username              string
	Password              string
	PassCredentialsAll    bool
	InsecureSkipTLSVerify bool
	CertFile              string
	KeyFile               string
	CAFile                string
	ChartVersion          string
}

type FindChartInRepoURLOption func(*findChartInRepoURLOptions)

// WithChartVersion specifies the chart version to find
func WithChartVersion(chartVersion string) FindChartInRepoURLOption {
	return func(options *findChartInRepoURLOptions) {
		options.ChartVersion = chartVersion
	}
}

// WithUsernamePassword specifies the username/password credntials for the repository
func WithUsernamePassword(username, password string) FindChartInRepoURLOption {
	return func(options *findChartInRepoURLOptions) {
		options.Username = username
		options.Password = password
	}
}

// WithPassCredentialsAll flags whether credentials should be passed on to other domains
func WithPassCredentialsAll(passCredentialsAll bool) FindChartInRepoURLOption {
	return func(options *findChartInRepoURLOptions) {
		options.PassCredentialsAll = passCredentialsAll
	}
}

// WithClientTLS species the cert, key, and CA files for client mTLS
func WithClientTLS(certFile, keyFile, caFile string) FindChartInRepoURLOption {
	return func(options *findChartInRepoURLOptions) {
		options.CertFile = certFile
		options.KeyFile = keyFile
		options.CAFile = caFile
	}
}

// WithInsecureSkipTLSVerify skips TLS verification for repository communication
func WithInsecureSkipTLSVerify(insecureSkipTLSVerify bool) FindChartInRepoURLOption {
	return func(options *findChartInRepoURLOptions) {
		options.InsecureSkipTLSVerify = insecureSkipTLSVerify
	}
}

// FindChartInRepoURL finds chart in chart repository pointed by repoURL
// without adding repo to repositories
func FindChartInRepoURL(repoURL string, chartName string, getters getter.Providers, options ...FindChartInRepoURLOption) (string, error) {

	opts := findChartInRepoURLOptions{}
	for _, option := range options {
		option(&opts)
	}

	// Download and write the index file to a temporary location
	buf := make([]byte, 20)
	rand.Read(buf)
	name := strings.ReplaceAll(base64.StdEncoding.EncodeToString(buf), "/", "-")

	c := Entry{
		URL:                   repoURL,
		Username:              opts.Username,
		Password:              opts.Password,
		PassCredentialsAll:    opts.PassCredentialsAll,
		CertFile:              opts.CertFile,
		KeyFile:               opts.KeyFile,
		CAFile:                opts.CAFile,
		Name:                  name,
		InsecureSkipTLSVerify: opts.InsecureSkipTLSVerify,
	}
	r, err := NewChartRepository(&c, getters)
	if err != nil {
		return "", err
	}
	idx, err := r.DownloadIndexFile()
	if err != nil {
		return "", fmt.Errorf("looks like %q is not a valid chart repository or cannot be reached: %w", repoURL, err)
	}
	defer func() {
		os.RemoveAll(filepath.Join(r.CachePath, helmpath.CacheChartsFile(r.Config.Name)))
		os.RemoveAll(filepath.Join(r.CachePath, helmpath.CacheIndexFile(r.Config.Name)))
	}()

	// Read the index file for the repository to get chart information and return chart URL
	repoIndex, err := LoadIndexFile(idx)
	if err != nil {
		return "", err
	}

	errMsg := fmt.Sprintf("chart %q", chartName)
	if opts.ChartVersion != "" {
		errMsg = fmt.Sprintf("%s version %q", errMsg, opts.ChartVersion)
	}
	cv, err := repoIndex.Get(chartName, opts.ChartVersion)
	if err != nil {
		return "", ChartNotFoundError{
			Chart:   errMsg,
			RepoURL: repoURL,
		}
	}

	if len(cv.URLs) == 0 {
		return "", fmt.Errorf("%s has no downloadable URLs", errMsg)
	}

	chartURL := cv.URLs[0]

	absoluteChartURL, err := ResolveReferenceURL(repoURL, chartURL)
	if err != nil {
		return "", fmt.Errorf("failed to make chart URL absolute: %w", err)
	}

	return absoluteChartURL, nil
}

// ResolveReferenceURL resolves refURL relative to baseURL.
// If refURL is absolute, it simply returns refURL.
func ResolveReferenceURL(baseURL, refURL string) (string, error) {
	parsedRefURL, err := url.Parse(refURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse %s as URL: %w", refURL, err)
	}

	if parsedRefURL.IsAbs() {
		return refURL, nil
	}

	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse %s as URL: %w", baseURL, err)
	}

	// We need a trailing slash for ResolveReference to work, but make sure there isn't already one
	parsedBaseURL.RawPath = strings.TrimSuffix(parsedBaseURL.RawPath, "/") + "/"
	parsedBaseURL.Path = strings.TrimSuffix(parsedBaseURL.Path, "/") + "/"

	resolvedURL := parsedBaseURL.ResolveReference(parsedRefURL)
	resolvedURL.RawQuery = parsedBaseURL.RawQuery
	return resolvedURL.String(), nil
}

func (e *Entry) String() string {
	buf, err := json.Marshal(e)
	if err != nil {
		slog.Error("failed to marshal entry", slog.Any("error", err))
		panic(err)
	}
	return string(buf)
}
