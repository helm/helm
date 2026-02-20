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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"helm.sh/helm/v4/pkg/chart/loader"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
)

// GitGetter handles fetching charts from Git repositories
type GitGetter struct {
	opts getterOptions
}

// Get performs a Get from a Git repository and returns the body.
func (g *GitGetter) Get(href string, options ...Option) (*bytes.Buffer, error) {
	for _, opt := range options {
		opt(&g.opts)
	}
	return g.get(href)
}

// get clones a Git repository, packages the chart, and returns it as a buffer
func (g *GitGetter) get(href string) (*bytes.Buffer, error) {
	// Parse the Git URL
	// Format: git://github.com/user/repo@ref?path=charts/mychart
	// Or: git+https://github.com/user/repo@ref?path=charts/mychart

	repoURL, ref, chartPath, err := parseGitURL(href)
	if err != nil {
		return nil, fmt.Errorf("failed to parse git URL: %w", err)
	}

	// Use version from options if provided (takes precedence over URL ref)
	// This allows the dependency version field to specify the Git ref
	if g.opts.version != "" && g.opts.version != "*" {
		ref = g.opts.version
	}

	// Create a temporary directory for cloning
	tmpDir, err := os.MkdirTemp("", "helm-git-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	slog.Debug("cloning git repository", "url", repoURL, "ref", ref, "path", chartPath)

	// Create context with timeout
	ctx, cancel := g.createContext()
	defer cancel()

	// Clone the repository
	if err := g.cloneRepo(ctx, repoURL, ref, tmpDir); err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Determine the chart directory
	chartDir := tmpDir
	if chartPath != "" {
		chartDir = filepath.Join(tmpDir, chartPath)
	}

	// Check if the chart directory exists
	if _, err := os.Stat(chartDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("chart path %q does not exist in repository", chartPath)
	}

	// Package the chart into a tarball
	tarData, err := g.packageChart(ctx, chartDir)
	if err != nil {
		return nil, fmt.Errorf("failed to package chart: %w", err)
	}

	return tarData, nil
}

// createContext creates a context with timeout from getter options
func (g *GitGetter) createContext() (context.Context, context.CancelFunc) {
	if g.opts.timeout > 0 {
		return context.WithTimeout(context.Background(), g.opts.timeout)
	}
	// Default timeout of 5 minutes for Git operations
	return context.WithTimeout(context.Background(), 5*time.Minute)
}

// cloneRepo clones a Git repository to the specified directory
func (g *GitGetter) cloneRepo(ctx context.Context, repoURL, ref, destDir string) error {
	// Use shallow clone for better performance
	args := []string{"clone", "--depth", "1"}

	// If a specific ref is provided, clone that branch/tag
	if ref != "" && ref != "HEAD" && ref != "master" && ref != "main" {
		args = append(args, "--branch", ref)
	}

	args = append(args, repoURL, destDir)

	cmd := exec.CommandContext(ctx, "git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Check if it was a timeout
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("git clone timed out")
		}

		// If shallow clone with branch failed, try full clone and checkout
		if ref != "" && ref != "HEAD" && ref != "master" && ref != "main" {
			slog.Debug("shallow clone failed, trying full clone", "error", stderr.String())
			return g.fullCloneAndCheckout(ctx, repoURL, ref, destDir)
		}
		return fmt.Errorf("git clone failed: %s", stderr.String())
	}

	// If ref is specified but wasn't used in clone (HEAD, master, main), checkout now
	if ref != "" && (ref == "HEAD" || ref == "master" || ref == "main") {
		cmd := exec.CommandContext(ctx, "git", "-C", destDir, "checkout", ref)
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("git checkout timed out")
			}
			return fmt.Errorf("git checkout failed: %s", stderr.String())
		}
	}

	return nil
}

// fullCloneAndCheckout performs a full clone and checks out a specific ref (commit SHA, tag, or branch)
func (g *GitGetter) fullCloneAndCheckout(ctx context.Context, repoURL, ref, destDir string) error {
	var stderr bytes.Buffer

	// Remove the directory if it exists from failed shallow clone
	os.RemoveAll(destDir)

	// Full clone
	cmd := exec.CommandContext(ctx, "git", "clone", repoURL, destDir)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("git clone timed out")
		}
		return fmt.Errorf("git clone failed: %s", stderr.String())
	}

	// Checkout the specific ref
	cmd = exec.CommandContext(ctx, "git", "-C", destDir, "checkout", ref)
	stderr.Reset()
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("git checkout timed out")
		}
		return fmt.Errorf("git checkout failed for ref %q: %s", ref, stderr.String())
	}

	return nil
}

// packageChart packages a chart directory into a tarball
func (g *GitGetter) packageChart(ctx context.Context, chartDir string) (*bytes.Buffer, error) {
	// Load the chart from the directory
	chrt, err := loader.LoadDir(chartDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load chart: %w", err)
	}

	// Type assert to get the v2 chart
	var ch *chart.Chart
	switch c := chrt.(type) {
	case *chart.Chart:
		ch = c
	case chart.Chart:
		ch = &c
	default:
		return nil, errors.New("invalid chart apiVersion")
	}

	// Create a temporary directory for the packaged chart
	tmpDir, err := os.MkdirTemp("", "helm-git-package-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory for packaging: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	packagePath, err := chartutil.Save(ch, tmpDir)
	if err != nil {
		return nil, fmt.Errorf("failed to package chart: %w", err)
	}

	tarData, err := os.ReadFile(packagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read package file: %w", err)
	}

	slog.Debug("Packaged chart hash", "hash", computeHash(tarData))

	return bytes.NewBuffer(tarData), nil
}

// parseGitURL parses a Git URL and extracts the repository URL, ref, and chart path
// Supported formats:
// - git://github.com/user/repo@ref?path=charts/mychart
// - git+https://github.com/user/repo@ref?path=charts/mychart
// - git+ssh://git@github.com/user/repo@ref?path=charts/mychart
func parseGitURL(href string) (repoURL, ref, chartPath string, err error) {
	u, err := url.Parse(href)
	if err != nil {
		return "", "", "", err
	}

	// Extract the ref from the URL fragment or path
	ref = "HEAD" // default ref
	repoPath := u.Path

	// Check if ref is specified with @ symbol
	if strings.Contains(u.Path, "@") {
		parts := strings.SplitN(u.Path, "@", 2)
		repoPath = parts[0]
		ref = parts[1]
	}

	// Check if ref is specified as a query parameter
	if u.Query().Get("ref") != "" {
		ref = u.Query().Get("ref")
	}

	// Extract chart path from query parameter
	chartPath = u.Query().Get("path")

	// Reconstruct the repository URL
	scheme := u.Scheme
	if strings.HasPrefix(scheme, "git+") {
		scheme = strings.TrimPrefix(scheme, "git+")
	} else if scheme == "git" {
		// Convert git:// to https:// for better compatibility
		scheme = "https"
	}

	// Handle git+ssh special case
	if scheme == "ssh" {
		// git+ssh://git@github.com/user/repo becomes git@github.com:user/repo
		if u.User != nil {
			username := u.User.Username()
			repoURL = fmt.Sprintf("%s@%s:%s", username, u.Host, strings.TrimPrefix(repoPath, "/"))
		} else {
			repoURL = fmt.Sprintf("git@%s:%s", u.Host, strings.TrimPrefix(repoPath, "/"))
		}
	} else {
		repoURL = fmt.Sprintf("%s://%s%s", scheme, u.Host, repoPath)
	}

	return repoURL, ref, chartPath, nil
}

// computeHash computes a SHA256 hash for the given data
func computeHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// NewGitGetter constructs a valid Git backend handler
func NewGitGetter(options ...Option) (Getter, error) {
	var client GitGetter

	for _, opt := range options {
		opt(&client.opts)
	}

	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("git command not found in PATH: %w", err)
	}

	return &client, nil
}
