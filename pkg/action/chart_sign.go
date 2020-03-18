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

package action

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/types"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/auth/challenge"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/theupdateframework/notary"
	"github.com/theupdateframework/notary/client"
	"github.com/theupdateframework/notary/cryptoservice"
	"github.com/theupdateframework/notary/passphrase"
	"github.com/theupdateframework/notary/trustmanager"
	"github.com/theupdateframework/notary/trustpinning"
	"github.com/theupdateframework/notary/tuf/data"
	"github.com/theupdateframework/notary/tuf/utils"
	"helm.sh/helm/v3/internal/experimental/registry"
	"helm.sh/helm/v3/pkg/helmpath"
)

// ChartPush performs a chart sign operation
type ChartSign struct {
	cfg         *Configuration
	trustDir    string
	trustServer string
	ref         string
	caCert      string
	rootKey     string
}

// NewChartPush creates a new ChartPush object with the given configuration.
func NewChartSign(cfg *Configuration, trustServer, ref, caCert, rootKey string) *ChartSign {
	return &ChartSign{
		cfg:         cfg,
		trustServer: trustServer,
		ref:         ref,
		caCert:      caCert,
		rootKey:     rootKey,
	}
}

// Run executes the chart push operation
func (a *ChartSign) Run(out io.Writer, ref string) error {

	// Init Registry Cache
	cacheDir := filepath.Join(helmpath.CachePath(), "registry", registry.CacheRootDir)
	cache, err := registry.NewCache(registry.CacheOptWriter(out), registry.CacheOptRoot(cacheDir))
	r, err := registry.ParseReference(ref)
	if err != nil {
		return err
	}

	cacheSummary, err := cache.FetchReference(r)
	if err != nil {
		return err
	}

	cachedChart := filepath.Join(cacheDir, "blobs", "sha256", strings.Split(cacheSummary.Digest.String(), ":")[1])

	/// Export to action and tuf experimental

	transport, err := MakeTransport(a.trustServer, r.Repo, a.caCert)
	if err != nil {
		return fmt.Errorf("cannot make transport: %v", err)
	}

	passphraseRetriever := passphrase.PromptRetriever()

	repo, err := client.NewFileCachedRepository(
		a.trustDir,
		data.GUN(r.Repo),
		a.trustServer,
		transport,
		passphraseRetriever,
		trustpinning.TrustPinConfig{},
	)

	if err != nil {
		return fmt.Errorf("cannot create new file cached repository: %v", err)
	}

	err = clearChangeList(repo)
	if err != nil {
		return fmt.Errorf("cannot clear change list: %v", err)
	}

	if _, err = repo.ListTargets(); err != nil {
		switch err.(type) {
		case client.ErrRepoNotInitialized, client.ErrRepositoryNotExist:
			rootKeyIDs, err := importRootKey(a.rootKey, repo, passphraseRetriever)
			if err != nil {
				return err
			}

			if err = repo.Initialize(rootKeyIDs); err != nil {
				return fmt.Errorf("cannot initialize repo: %v", err)
			}

		default:
			return fmt.Errorf("cannot list targets: %v", err)
		}
	}

	target, err := client.NewTarget(r.Tag, cachedChart, nil)
	if err != nil {
		return err
	}

	// TODO - Radu M
	// decide whether to allow actually passing roles as flags

	// If roles is empty, we default to adding to targets
	if err = repo.AddTarget(target, data.NewRoleList([]string{})...); err != nil {
		return err
	}

	err = repo.Publish()

	defer clearChangeList(repo)

	return err
}

func MakeTransport(server, gun, tlsCaCert string) (http.RoundTripper, error) {
	modifiers := []transport.RequestModifier{
		transport.NewHeaderRequestModifier(http.Header{
			"User-Agent": []string{"signy"},
		}),
	}

	base := http.DefaultTransport
	if tlsCaCert != "" {
		caCert, err := ioutil.ReadFile(tlsCaCert)
		if err != nil {
			return nil, fmt.Errorf("cannot read cert file: %v", err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		base = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		}
	}

	authTransport := transport.NewTransport(base, modifiers...)
	pingClient := &http.Client{
		Transport: authTransport,
		Timeout:   5 * time.Second,
	}
	req, err := http.NewRequest("GET", server+"/v2/", nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP request: %v", err)
	}

	challengeManager := challenge.NewSimpleManager()
	resp, err := pingClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot get response from ping client: %v", err)
	}
	defer resp.Body.Close()
	if err := challengeManager.AddResponse(resp); err != nil {
		return nil, fmt.Errorf("cannot add response to challenge manager: %v", err)
	}

	defaultAuth, err := getDefaultAuth()
	if err != nil {
		return nil, fmt.Errorf("cannot get default credentials: %v", err)
	}

	creds := simpleCredentialStore{auth: defaultAuth}
	tokenHandler := auth.NewTokenHandler(base, creds, gun, "push", "pull")
	modifiers = append(modifiers, auth.NewAuthorizer(challengeManager, tokenHandler))

	return transport.NewTransport(base, modifiers...), nil
}

// clearChangelist clears the notary staging changelist
func clearChangeList(notaryRepo client.Repository) error {
	cl, err := notaryRepo.GetChangelist()
	if err != nil {
		return err
	}
	return cl.Clear("")
}

func getDefaultAuth() (types.AuthConfig, error) {
	cfg, err := config.Load(defaultCfgDir())
	if err != nil {
		return types.AuthConfig{}, err
	}

	return cfg.AuthConfigs["https://index.docker.io/v1/"], nil
}

type simpleCredentialStore struct {
	auth types.AuthConfig
}

func (scs simpleCredentialStore) Basic(u *url.URL) (string, string) {
	return scs.auth.Username, scs.auth.Password
}

func (scs simpleCredentialStore) RefreshToken(u *url.URL, service string) string {
	return scs.auth.IdentityToken
}

func (scs simpleCredentialStore) SetRefreshToken(*url.URL, string, string) {
}

func defaultCfgDir() string {
	homeEnvPath := os.Getenv("HOME")
	if homeEnvPath == "" && runtime.GOOS == "windows" {
		homeEnvPath = os.Getenv("USERPROFILE")
	}

	return filepath.Join(homeEnvPath, ".docker")
}

func importRootKey(rootKey string, nRepo client.Repository, retriever notary.PassRetriever) ([]string, error) {
	var rootKeyList []string

	if rootKey != "" {
		privKey, err := readKey(data.CanonicalRootRole, rootKey, retriever)
		if err != nil {
			return nil, err
		}
		// add root key to repo
		err = nRepo.GetCryptoService().AddKey(data.CanonicalRootRole, "", privKey)
		if err != nil {
			return nil, fmt.Errorf("Error importing key: %v", err)
		}
		rootKeyList = []string{privKey.ID()}
	} else {
		rootKeyList = nRepo.GetCryptoService().ListKeys(data.CanonicalRootRole)
	}

	if len(rootKeyList) > 0 {
		// Chooses the first root key available, which is initialization specific
		// but should return the HW one first.
		rootKeyID := rootKeyList[0]
		fmt.Printf("Root key found, using: %s\n", rootKeyID)

		return []string{rootKeyID}, nil
	}

	return []string{}, nil
}

func readKey(role data.RoleName, keyFilename string, retriever notary.PassRetriever) (data.PrivateKey, error) {
	pemBytes, err := ioutil.ReadFile(keyFilename)
	if err != nil {
		return nil, fmt.Errorf("Error reading input root key file: %v", err)
	}
	isEncrypted := true
	if err = cryptoservice.CheckRootKeyIsEncrypted(pemBytes); err != nil {
		if role == data.CanonicalRootRole {
			return nil, err
		}
		isEncrypted = false
	}
	var privKey data.PrivateKey
	if isEncrypted {
		privKey, _, err = trustmanager.GetPasswdDecryptBytes(retriever, pemBytes, "", data.CanonicalRootRole.String())
	} else {
		privKey, err = utils.ParsePEMPrivateKey(pemBytes, "")
	}
	if err != nil {
		return nil, err
	}

	return privKey, nil
}
