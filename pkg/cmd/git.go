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

package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/internal/gitupdate"
	"helm.sh/helm/v4/pkg/cmd/require"
)

const gitDesc = `
Work with files stored in Git repositories.

Git operations are implemented inside the Helm binary and do not require a
separate git executable.
`

const gitUpdateDesc = `
Modify one file in a Git repository, commit the change, and push it.

The edit is deterministic: text selectors fail unless they match the expected
number of locations, and structured edits use RFC 6901 JSON pointers. An empty
pointer addresses the document root. For example:

    helm git update https://example.com/platform/config.git \
      --branch main --file charts/api/values.yaml \
      --method yaml-set --path /image/tag --content 1.4.2

    helm git update git@example.com:platform/config.git \
      --branch main --file deploy/app.yaml \
      --method replace --match 'replicas: 2' --content 'replicas: 3' \
      --auth ssh-agent

The default --transport=clone backend uses the Git smart protocol and a
temporary worktree. The --transport=api backend uses a repository service API
to read and commit the file without cloning. GitLab is the default API
provider:

    helm git update https://gitlab.example.com/platform/config.git \
      --transport api --api-provider gitlab --token-file /run/secrets/git-token \
      --branch main --file charts/api/values.yaml \
      --method yaml-set --path /image/tag --content 1.4.2

Content can be supplied with --content, --content-env, --content-file, or
--content-stdin. Secret flags also read HELM_GIT_PASSWORD, HELM_GIT_TOKEN, and
HELM_GIT_SSH_KEY_PASSPHRASE when their command-line and file forms are absent.
`

type gitUpdateOptions struct {
	repositoryURL string
	transport     string
	branch        string
	baseBranch    string
	createBranch  bool
	targetFile    string
	createFile    bool
	method        string

	content      string
	contentEnv   string
	contentFile  string
	contentStdin bool
	match        string
	matchFile    string
	regex        string
	pointer      string
	createPath   bool
	expected     int
	all          bool
	expand       bool

	authType             string
	username             string
	password             string
	passwordFile         string
	token                string
	tokenFile            string
	sshPrivateKey        string
	sshKeyPassphrase     string
	sshPassphraseFile    string
	sshKnownHosts        string
	insecureSSHHostKey   bool
	caFile               string
	certFile             string
	keyFile              string
	insecureSkipTLS      bool
	apiProvider          string
	apiBaseURL           string
	apiProject           string
	apiTokenType         string
	authorName           string
	authorEmail          string
	commitMessage        string
	depth                int
	dryRun               bool
	push                 bool
	resolvedContent      []byte
	contentWasProvided   bool
	resolvedLiteralMatch string
}

func newGitCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "git",
		Short:             "edit files in Git repositories",
		Long:              gitDesc,
		Args:              require.NoArgs,
		ValidArgsFunction: noMoreArgsCompFunc,
	}
	cmd.AddCommand(newGitUpdateCmd(out))
	return cmd
}

func newGitUpdateCmd(out io.Writer) *cobra.Command {
	o := &gitUpdateOptions{}
	if value := os.Getenv("GIT_AUTHOR_NAME"); value != "" {
		o.authorName = value
	} else {
		o.authorName = "Helm Git Update"
	}
	if value := os.Getenv("GIT_AUTHOR_EMAIL"); value != "" {
		o.authorEmail = value
	} else {
		o.authorEmail = "helm-git@localhost"
	}

	cmd := &cobra.Command{
		Use:   "update REPOSITORY",
		Short: "edit, commit, and push a file in a Git repository",
		Long:  gitUpdateDesc,
		Args:  require.ExactArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return noMoreArgsComp()
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return o.complete(cmd, args)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return o.run(cmd, out)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&o.transport, "transport", gitupdate.TransportClone, "remote update backend ("+strings.Join(gitupdate.ValidTransports(), ", ")+")")
	flags.StringVar(&o.branch, "branch", "", "target branch to update")
	flags.StringVar(&o.baseBranch, "base-branch", "", "branch from which a new target branch is created")
	flags.BoolVar(&o.createBranch, "create-branch", false, "create the target branch from --base-branch")
	flags.StringVar(&o.targetFile, "file", "", "file to edit, relative to the repository root")
	flags.BoolVar(&o.createFile, "create-file", false, "create the target file and parent directories when absent")
	flags.StringVar(&o.method, "method", string(gitupdate.MethodOverwrite), "edit method ("+strings.Join(gitupdate.ValidMethods(), ", ")+")")

	flags.StringVar(&o.content, "content", "", "new content or structured value")
	flags.StringVar(&o.contentEnv, "content-env", "", "read new content or structured value from the named environment variable")
	flags.StringVar(&o.contentFile, "content-file", "", "read new content or structured value from a file")
	flags.BoolVar(&o.contentStdin, "content-stdin", false, "read new content or structured value from standard input")
	flags.StringVar(&o.match, "match", "", "literal text selector for replace or insert methods")
	flags.StringVar(&o.matchFile, "match-file", "", "read the literal text selector from a file")
	flags.StringVar(&o.regex, "regex", "", "Go regular expression selector for replace or insert methods")
	flags.IntVar(&o.expected, "expected-matches", 1, "required selector match count; 0 accepts any non-zero count")
	flags.BoolVar(&o.all, "all", false, "edit all selector matches instead of only the first")
	flags.BoolVar(&o.expand, "expand", false, "expand regular expression captures such as $1 in content")
	flags.StringVar(&o.pointer, "path", "", "RFC 6901 path used by YAML and JSON set, merge, and delete methods")
	flags.BoolVar(&o.createPath, "create-path", false, "create absent mapping components for structured set or merge")

	flags.StringVar(&o.authType, "auth", gitupdate.AuthAuto, "authentication mode ("+strings.Join(gitupdate.ValidAuthTypes(), ", ")+")")
	flags.StringVar(&o.username, "username", os.Getenv("HELM_GIT_USERNAME"), "HTTP or SSH username")
	flags.StringVar(&o.password, "password", "", "HTTP or SSH password (prefer --password-file or HELM_GIT_PASSWORD)")
	flags.StringVar(&o.passwordFile, "password-file", "", "read the HTTP or SSH password from a file")
	flags.StringVar(&o.token, "token", "", "HTTP token (prefer --token-file or HELM_GIT_TOKEN)")
	flags.StringVar(&o.tokenFile, "token-file", "", "read the HTTP token from a file")
	flags.StringVar(&o.sshPrivateKey, "ssh-private-key", "", "SSH private key file")
	flags.StringVar(&o.sshKeyPassphrase, "ssh-key-passphrase", "", "SSH key passphrase (prefer its file or environment form)")
	flags.StringVar(&o.sshPassphraseFile, "ssh-key-passphrase-file", "", "read the SSH key passphrase from a file")
	flags.StringVar(&o.sshKnownHosts, "ssh-known-hosts", "", "SSH known_hosts file (defaults to standard SSH known_hosts files)")
	flags.BoolVar(&o.insecureSSHHostKey, "insecure-skip-ssh-host-key", false, "skip SSH host-key verification (insecure)")

	flags.StringVar(&o.caFile, "ca-file", "", "CA bundle used to verify the Git HTTPS server")
	flags.StringVar(&o.certFile, "cert-file", "", "client certificate used for Git mutual TLS")
	flags.StringVar(&o.keyFile, "key-file", "", "client key used for Git mutual TLS")
	flags.BoolVar(&o.insecureSkipTLS, "insecure-skip-tls-verify", false, "skip Git HTTPS certificate verification (insecure)")

	flags.StringVar(&o.apiProvider, "api-provider", gitupdate.APIProviderAuto, "repository API provider ("+strings.Join(gitupdate.ValidAPIProviders(), ", ")+")")
	flags.StringVar(&o.apiBaseURL, "api-base-url", "", "repository API base URL (inferred from REPOSITORY)")
	flags.StringVar(&o.apiProject, "api-project", "", "repository API project ID or path (inferred from REPOSITORY)")
	flags.StringVar(&o.apiTokenType, "api-token-type", gitupdate.APITokenAuto, "repository API token type ("+strings.Join(gitupdate.ValidAPITokenTypes(), ", ")+")")

	flags.StringVar(&o.authorName, "author-name", o.authorName, "commit author name")
	flags.StringVar(&o.authorEmail, "author-email", o.authorEmail, "commit author email")
	flags.StringVarP(&o.commitMessage, "message", "m", "", "commit message (defaults to 'helm git update <file>')")
	flags.IntVar(&o.depth, "depth", 1, "clone depth; 0 clones the complete history")
	flags.BoolVar(&o.dryRun, "dry-run", false, "read and validate the edit without committing or pushing")
	flags.BoolVar(&o.push, "push", true, "push the commit to the remote repository")

	_ = cmd.MarkFlagRequired("branch")
	_ = cmd.MarkFlagRequired("file")
	_ = cmd.RegisterFlagCompletionFunc("method", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return gitupdate.ValidMethods(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("auth", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return gitupdate.ValidAuthTypes(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("transport", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return gitupdate.ValidTransports(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("api-provider", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return gitupdate.ValidAPIProviders(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("api-token-type", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return gitupdate.ValidAPITokenTypes(), cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func (o *gitUpdateOptions) complete(cmd *cobra.Command, args []string) error {
	o.repositoryURL = args[0]
	if o.commitMessage == "" {
		o.commitMessage = "helm git update " + o.targetFile
	}

	contentSources := 0
	if cmd.Flags().Changed("content") {
		contentSources++
		o.resolvedContent = []byte(o.content)
		o.contentWasProvided = true
	}
	if o.contentEnv != "" {
		contentSources++
		content, exists := os.LookupEnv(o.contentEnv)
		if !exists {
			return fmt.Errorf("content environment variable %q is not set", o.contentEnv)
		}
		o.resolvedContent = []byte(content)
		o.contentWasProvided = true
	}
	if o.contentFile != "" {
		contentSources++
		content, err := os.ReadFile(o.contentFile)
		if err != nil {
			return fmt.Errorf("read content file: %w", err)
		}
		o.resolvedContent = content
		o.contentWasProvided = true
	}
	if o.contentStdin {
		contentSources++
		content, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return fmt.Errorf("read content from standard input: %w", err)
		}
		o.resolvedContent = content
		o.contentWasProvided = true
	}
	if contentSources > 1 {
		return errors.New("use only one of --content, --content-env, --content-file, or --content-stdin")
	}

	o.resolvedLiteralMatch = o.match
	if o.match != "" && o.matchFile != "" {
		return errors.New("use only one of --match or --match-file")
	}
	if o.matchFile != "" {
		match, err := os.ReadFile(o.matchFile)
		if err != nil {
			return fmt.Errorf("read match file: %w", err)
		}
		o.resolvedLiteralMatch = string(match)
	}

	password, err := resolveCredential(o.password, o.passwordFile, "HELM_GIT_PASSWORD")
	if err != nil {
		return fmt.Errorf("resolve password: %w", err)
	}
	o.password = password
	token, err := resolveCredential(o.token, o.tokenFile, "HELM_GIT_TOKEN")
	if err != nil {
		return fmt.Errorf("resolve token: %w", err)
	}
	o.token = token
	passphrase, err := resolveCredential(o.sshKeyPassphrase, o.sshPassphraseFile, "HELM_GIT_SSH_KEY_PASSPHRASE")
	if err != nil {
		return fmt.Errorf("resolve SSH key passphrase: %w", err)
	}
	o.sshKeyPassphrase = passphrase
	return nil
}

func (o *gitUpdateOptions) run(cmd *cobra.Command, out io.Writer) error {
	var progress io.Writer
	if settings.Debug {
		progress = cmd.ErrOrStderr()
	}

	result, err := gitupdate.Update(cmd.Context(), gitupdate.RepositoryOptions{
		RepositoryURL: o.repositoryURL,
		Transport:     o.transport,
		Branch:        o.branch,
		BaseBranch:    o.baseBranch,
		CreateBranch:  o.createBranch,
		TargetFile:    o.targetFile,
		CreateFile:    o.createFile,
		Edit: gitupdate.EditOptions{
			Method:          gitupdate.Method(o.method),
			Content:         o.resolvedContent,
			ContentProvided: o.contentWasProvided,
			LiteralMatch:    o.resolvedLiteralMatch,
			RegexMatch:      o.regex,
			ExpectedMatches: o.expected,
			AllMatches:      o.all,
			Expand:          o.expand,
			Pointer:         o.pointer,
			CreatePath:      o.createPath,
		},
		Auth: gitupdate.AuthOptions{
			Type:                o.authType,
			Username:            o.username,
			Password:            o.password,
			Token:               o.token,
			SSHPrivateKey:       o.sshPrivateKey,
			SSHKeyPassphrase:    o.sshKeyPassphrase,
			KnownHostsFile:      o.sshKnownHosts,
			InsecureHostKey:     o.insecureSSHHostKey,
			SSHAgentSocketIsSet: gitupdate.AgentAvailable(),
		},
		TLS: gitupdate.TLSOptions{
			CAFile:            o.caFile,
			ClientCertificate: o.certFile,
			ClientKey:         o.keyFile,
			InsecureSkipTLS:   o.insecureSkipTLS,
		},
		API: gitupdate.APIOptions{
			Provider:  o.apiProvider,
			BaseURL:   o.apiBaseURL,
			Project:   o.apiProject,
			TokenType: o.apiTokenType,
		},
		AuthorName:    o.authorName,
		AuthorEmail:   o.authorEmail,
		CommitMessage: o.commitMessage,
		Depth:         o.depth,
		DryRun:        o.dryRun,
		Push:          o.push,
		Progress:      progress,
	})
	if err != nil {
		return err
	}

	switch {
	case !result.Changed:
		fmt.Fprintf(out, "No changes needed for %s on branch %s\n", result.TargetFile, result.Branch)
	case o.dryRun:
		fmt.Fprintf(out, "Would update %s on branch %s using %s transport (dry run; no remote commit)\n", result.TargetFile, result.Branch, result.Transport)
	case result.Pushed:
		fmt.Fprintf(out, "Updated %s on branch %s using %s transport in commit %s\n", result.TargetFile, result.Branch, result.Transport, result.CommitHash)
	case result.Committed:
		fmt.Fprintf(out, "Updated %s on branch %s in commit %s (not pushed)\n", result.TargetFile, result.Branch, result.CommitHash)
	}
	return nil
}

func resolveCredential(direct, filename, environment string) (string, error) {
	if direct != "" && filename != "" {
		return "", errors.New("a direct value and file cannot both be set")
	}
	if filename != "" {
		value, err := os.ReadFile(filename)
		if err != nil {
			return "", err
		}
		return trimOneLineEnding(string(value)), nil
	}
	if direct != "" {
		return direct, nil
	}
	return os.Getenv(environment), nil
}

func trimOneLineEnding(value string) string {
	value = strings.TrimSuffix(value, "\n")
	value = strings.TrimSuffix(value, "\r")
	return value
}
