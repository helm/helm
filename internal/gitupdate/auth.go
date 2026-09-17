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
	"errors"
	"fmt"
	"os"
	"strings"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"golang.org/x/crypto/ssh"

	"github.com/go-git/go-git/v5/plumbing/transport"
)

const (
	AuthAuto        = "auto"
	AuthNone        = "none"
	AuthBasic       = "basic"
	AuthToken       = "token"
	AuthBearer      = "bearer"
	AuthSSHKey      = "ssh-key"
	AuthSSHAgent    = "ssh-agent"
	AuthSSHPassword = "ssh-password"
)

// ValidAuthTypes returns the authentication modes understood by BuildAuth.
func ValidAuthTypes() []string {
	return []string{
		AuthAuto,
		AuthNone,
		AuthBasic,
		AuthToken,
		AuthBearer,
		AuthSSHKey,
		AuthSSHAgent,
		AuthSSHPassword,
	}
}

// AuthOptions contains credentials for Git HTTP(S) and SSH transports.
type AuthOptions struct {
	Type                string
	Username            string
	Password            string
	Token               string
	SSHPrivateKey       string
	SSHKeyPassphrase    string
	KnownHostsFile      string
	InsecureHostKey     bool
	SSHAgentSocketIsSet bool
}

// BuildAuth constructs a go-git authentication method. Automatic mode chooses
// an explicit credential first, then an SSH agent for SSH URLs.
func BuildAuth(repositoryURL string, opts AuthOptions) (transport.AuthMethod, error) {
	authType := opts.Type
	if authType == "" {
		authType = AuthAuto
	}
	if authType == AuthAuto {
		switch {
		case opts.SSHPrivateKey != "":
			authType = AuthSSHKey
		case opts.Token != "":
			authType = AuthToken
		case opts.Password != "" && isSSHURL(repositoryURL):
			authType = AuthSSHPassword
		case opts.Password != "" || opts.Username != "":
			authType = AuthBasic
		case isSSHURL(repositoryURL) && opts.SSHAgentSocketIsSet:
			authType = AuthSSHAgent
		default:
			authType = AuthNone
		}
	}

	sshTransport := isSSHURL(repositoryURL)
	httpTransport := strings.HasPrefix(repositoryURL, "http://") || strings.HasPrefix(repositoryURL, "https://")
	switch authType {
	case AuthBasic, AuthToken, AuthBearer:
		if !httpTransport {
			return nil, fmt.Errorf("authentication mode %q requires an HTTP(S) repository URL", authType)
		}
	case AuthSSHKey, AuthSSHAgent, AuthSSHPassword:
		if !sshTransport {
			return nil, fmt.Errorf("authentication mode %q requires an SSH repository URL", authType)
		}
	}

	switch authType {
	case AuthNone:
		if opts.Username != "" || opts.Password != "" || opts.Token != "" || opts.SSHPrivateKey != "" || opts.SSHKeyPassphrase != "" {
			return nil, errors.New("authentication mode \"none\" cannot be combined with credentials")
		}
		if opts.KnownHostsFile != "" || opts.InsecureHostKey {
			return nil, errors.New("SSH host-key options require ssh-key, ssh-agent, or ssh-password authentication")
		}
		return nil, nil
	case AuthBasic:
		if opts.Username == "" {
			return nil, errors.New("basic authentication requires --username")
		}
		if opts.Password == "" {
			return nil, errors.New("basic authentication requires a password")
		}
		return &githttp.BasicAuth{Username: opts.Username, Password: opts.Password}, nil
	case AuthToken:
		if opts.Token == "" {
			return nil, errors.New("token authentication requires a token")
		}
		username := opts.Username
		if username == "" {
			username = "git"
		}
		return &githttp.BasicAuth{Username: username, Password: opts.Token}, nil
	case AuthBearer:
		if opts.Token == "" {
			return nil, errors.New("bearer authentication requires a token")
		}
		return &githttp.TokenAuth{Token: opts.Token}, nil
	case AuthSSHKey:
		if opts.SSHPrivateKey == "" {
			return nil, errors.New("ssh-key authentication requires --ssh-private-key")
		}
		username := sshUsername(opts.Username)
		key, err := gitssh.NewPublicKeysFromFile(username, opts.SSHPrivateKey, opts.SSHKeyPassphrase)
		if err != nil {
			return nil, fmt.Errorf("load SSH private key: %w", err)
		}
		if err := configureHostKeyCallback(&key.HostKeyCallbackHelper, opts); err != nil {
			return nil, err
		}
		return key, nil
	case AuthSSHAgent:
		username := sshUsername(opts.Username)
		agent, err := gitssh.NewSSHAgentAuth(username)
		if err != nil {
			return nil, fmt.Errorf("connect to SSH agent: %w", err)
		}
		if err := configureHostKeyCallback(&agent.HostKeyCallbackHelper, opts); err != nil {
			return nil, err
		}
		return agent, nil
	case AuthSSHPassword:
		if opts.Password == "" {
			return nil, errors.New("ssh-password authentication requires a password")
		}
		password := &gitssh.Password{User: sshUsername(opts.Username), Password: opts.Password}
		if err := configureHostKeyCallback(&password.HostKeyCallbackHelper, opts); err != nil {
			return nil, err
		}
		return password, nil
	default:
		return nil, fmt.Errorf("unknown authentication mode %q (supported: %s)", authType, strings.Join(ValidAuthTypes(), ", "))
	}
}

func configureHostKeyCallback(helper *gitssh.HostKeyCallbackHelper, opts AuthOptions) error {
	if opts.InsecureHostKey && opts.KnownHostsFile != "" {
		return errors.New("--insecure-skip-ssh-host-key cannot be combined with --ssh-known-hosts")
	}
	if opts.InsecureHostKey {
		helper.HostKeyCallback = ssh.InsecureIgnoreHostKey()
		return nil
	}
	if opts.KnownHostsFile != "" {
		callback, err := gitssh.NewKnownHostsCallback(opts.KnownHostsFile)
		if err != nil {
			return fmt.Errorf("load SSH known-hosts file: %w", err)
		}
		helper.HostKeyCallback = callback
	}
	return nil
}

func isSSHURL(repositoryURL string) bool {
	if strings.HasPrefix(repositoryURL, "ssh://") || strings.HasPrefix(repositoryURL, "git+ssh://") {
		return true
	}
	at := strings.IndexByte(repositoryURL, '@')
	colon := strings.IndexByte(repositoryURL, ':')
	return at > 0 && colon > at+1 && !strings.Contains(repositoryURL[:colon], "://")
}

func sshUsername(username string) string {
	if username == "" {
		return gitssh.DefaultUsername
	}
	return username
}

// AgentAvailable reports whether an SSH agent socket is configured.
func AgentAvailable() bool {
	return os.Getenv("SSH_AUTH_SOCK") != ""
}
