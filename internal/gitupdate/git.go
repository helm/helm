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
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const (
	// TransportClone uses the Git smart protocol to clone, commit, and push.
	TransportClone = "clone"
	// TransportAPI uses the repository provider's HTTP API without cloning.
	TransportAPI = "api"
)

// ValidTransports returns the supported remote update backends.
func ValidTransports() []string {
	return []string{TransportClone, TransportAPI}
}

// TLSOptions controls HTTPS server and client certificate handling.
type TLSOptions struct {
	CAFile            string
	ClientCertificate string
	ClientKey         string
	InsecureSkipTLS   bool
}

type loadedTLSOptions struct {
	caBundle          []byte
	clientCertificate []byte
	clientKey         []byte
	insecureSkipTLS   bool
}

// RepositoryOptions describes a remote edit-commit transaction.
type RepositoryOptions struct {
	RepositoryURL string
	Transport     string
	Branch        string
	BaseBranch    string
	CreateBranch  bool
	TargetFile    string
	CreateFile    bool
	Edit          EditOptions
	Auth          AuthOptions
	TLS           TLSOptions
	API           APIOptions
	AuthorName    string
	AuthorEmail   string
	CommitMessage string
	Depth         int
	DryRun        bool
	Push          bool
	Progress      io.Writer
	Now           func() time.Time
}

// Result summarizes a repository update.
type Result struct {
	Changed    bool
	Committed  bool
	Pushed     bool
	CommitHash string
	TargetFile string
	Branch     string
	Transport  string
}

// Update applies exactly one file edit and commits it through either a private
// temporary clone or the repository provider API. It never invokes the git
// executable.
func Update(ctx context.Context, opts RepositoryOptions) (Result, error) {
	if opts.Transport == "" {
		opts.Transport = TransportClone
	}
	result := Result{TargetFile: opts.TargetFile, Branch: opts.Branch, Transport: opts.Transport}
	if err := validateRepositoryOptions(opts); err != nil {
		return result, err
	}

	targetFile, err := validateTargetPath(opts.TargetFile)
	if err != nil {
		return result, err
	}
	result.TargetFile = targetFile

	if opts.Transport == TransportAPI {
		return updateViaAPI(ctx, opts, result)
	}

	auth, err := BuildAuth(opts.RepositoryURL, opts.Auth)
	if err != nil {
		return result, fmt.Errorf("configure Git authentication: %w", err)
	}
	tlsOptions, err := opts.TLS.load()
	if err != nil {
		return result, err
	}

	cloneBranch := opts.Branch
	if opts.CreateBranch {
		cloneBranch = opts.BaseBranch
	}
	cloneReference := plumbing.NewBranchReferenceName(cloneBranch)

	worktreeDir, err := os.MkdirTemp("", "helm-git-update-*")
	if err != nil {
		return result, fmt.Errorf("create temporary worktree: %w", err)
	}
	slog.Debug("created temporary Git worktree", "path", worktreeDir)
	defer os.RemoveAll(worktreeDir)

	repository, err := git.PlainCloneContext(ctx, worktreeDir, false, &git.CloneOptions{
		URL:             opts.RepositoryURL,
		Auth:            auth,
		ReferenceName:   cloneReference,
		SingleBranch:    !opts.CreateBranch,
		Depth:           opts.Depth,
		Progress:        opts.Progress,
		InsecureSkipTLS: tlsOptions.insecureSkipTLS,
		ClientCert:      tlsOptions.clientCertificate,
		ClientKey:       tlsOptions.clientKey,
		CABundle:        tlsOptions.caBundle,
	})
	if err != nil {
		return result, fmt.Errorf("clone branch %q: %w", cloneBranch, err)
	}

	worktree, err := repository.Worktree()
	if err != nil {
		return result, fmt.Errorf("open cloned worktree: %w", err)
	}
	if opts.CreateBranch {
		remoteTarget := plumbing.NewRemoteReferenceName("origin", opts.Branch)
		if _, err := repository.Reference(remoteTarget, true); err == nil {
			return result, fmt.Errorf("cannot create branch %q: it already exists on the remote", opts.Branch)
		} else if !errors.Is(err, plumbing.ErrReferenceNotFound) {
			return result, fmt.Errorf("check remote branch %q: %w", opts.Branch, err)
		}

		targetReference := plumbing.NewBranchReferenceName(opts.Branch)
		if err := worktree.Checkout(&git.CheckoutOptions{Branch: targetReference, Create: true}); err != nil {
			return result, fmt.Errorf("create branch %q from %q: %w", opts.Branch, opts.BaseBranch, err)
		}
	}

	changed, err := ApplyFile(worktreeDir, targetFile, opts.Edit, opts.CreateFile)
	if err != nil {
		return result, err
	}
	result.Changed = changed
	if !changed || opts.DryRun {
		return result, nil
	}

	if _, err := worktree.Add(targetFile); err != nil {
		return result, fmt.Errorf("stage target file %q: %w", targetFile, err)
	}

	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	signature := &object.Signature{
		Name:  opts.AuthorName,
		Email: opts.AuthorEmail,
		When:  now(),
	}
	hash, err := worktree.Commit(opts.CommitMessage, &git.CommitOptions{
		Author:    signature,
		Committer: signature,
	})
	if err != nil {
		return result, fmt.Errorf("commit target file %q: %w", targetFile, err)
	}
	result.Committed = true
	result.CommitHash = hash.String()

	if !opts.Push {
		return result, nil
	}
	targetReference := plumbing.NewBranchReferenceName(opts.Branch)
	refspec := config.RefSpec(fmt.Sprintf("%s:%s", targetReference, targetReference))
	if err := repository.PushContext(ctx, &git.PushOptions{
		Auth:            auth,
		RefSpecs:        []config.RefSpec{refspec},
		Progress:        opts.Progress,
		InsecureSkipTLS: tlsOptions.insecureSkipTLS,
		ClientCert:      tlsOptions.clientCertificate,
		ClientKey:       tlsOptions.clientKey,
		CABundle:        tlsOptions.caBundle,
	}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return result, fmt.Errorf("push branch %q: %w", opts.Branch, err)
	}
	result.Pushed = true
	return result, nil
}

func validateRepositoryOptions(opts RepositoryOptions) error {
	if opts.RepositoryURL == "" {
		return errors.New("repository URL is required")
	}
	transport := opts.Transport
	if transport == "" {
		transport = TransportClone
	}
	if !slices.Contains(ValidTransports(), transport) {
		return fmt.Errorf("unknown transport %q (supported: %s)", transport, strings.Join(ValidTransports(), ", "))
	}
	if _, err := validateTargetPath(opts.TargetFile); err != nil {
		return err
	}
	if err := validateEditOptions(opts.Edit); err != nil {
		return fmt.Errorf("invalid edit: %w", err)
	}
	if opts.Branch == "" {
		return errors.New("target branch is required")
	}
	if err := plumbing.NewBranchReferenceName(opts.Branch).Validate(); err != nil {
		return fmt.Errorf("invalid target branch %q: %w", opts.Branch, err)
	}
	if opts.CreateBranch {
		if opts.BaseBranch == "" {
			return errors.New("--base-branch is required with --create-branch")
		}
		if opts.BaseBranch == opts.Branch {
			return errors.New("target branch and base branch must differ when creating a branch")
		}
		if err := plumbing.NewBranchReferenceName(opts.BaseBranch).Validate(); err != nil {
			return fmt.Errorf("invalid base branch %q: %w", opts.BaseBranch, err)
		}
	} else if opts.BaseBranch != "" {
		return errors.New("--base-branch requires --create-branch")
	}
	if opts.AuthorName == "" {
		return errors.New("commit author name is required")
	}
	if opts.AuthorEmail == "" {
		return errors.New("commit author email is required")
	}
	if opts.CommitMessage == "" {
		return errors.New("commit message is required")
	}
	if opts.Depth < 0 {
		return errors.New("clone depth must be zero or greater")
	}
	if transport == TransportAPI {
		if err := validateAPIOptions(opts); err != nil {
			return err
		}
	}
	return nil
}

func (opts TLSOptions) load() (loadedTLSOptions, error) {
	loaded := loadedTLSOptions{insecureSkipTLS: opts.InsecureSkipTLS}
	if (opts.ClientCertificate == "") != (opts.ClientKey == "") {
		return loaded, errors.New("--git-cert-file and --git-key-file must be set together")
	}

	var err error
	if opts.CAFile != "" {
		loaded.caBundle, err = os.ReadFile(opts.CAFile)
		if err != nil {
			return loaded, fmt.Errorf("read Git CA file: %w", err)
		}
	}
	if opts.ClientCertificate != "" {
		loaded.clientCertificate, err = os.ReadFile(opts.ClientCertificate)
		if err != nil {
			return loaded, fmt.Errorf("read Git client certificate: %w", err)
		}
		loaded.clientKey, err = os.ReadFile(opts.ClientKey)
		if err != nil {
			return loaded, fmt.Errorf("read Git client key: %w", err)
		}
	}
	return loaded, nil
}
