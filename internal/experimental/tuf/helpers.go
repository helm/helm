// Most of the helper functions are adapted from github.com/theupdateframework/notary
//
// Figure out the proper way of making sure we are respecting the licensing from Notary
// While we are also vendoring Notary directly (see LICENSE in vendor/github.com/theupdateframework/notary/LICENSE),
// copying unexported functions could fall under different licensing, so we need to make sure.

package tuf

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
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
	"github.com/theupdateframework/notary/tuf/data"
	"github.com/theupdateframework/notary/tuf/utils"
)

const (
	configFileDir      = ".docker"
	defaultIndexServer = "https://index.docker.io/v1/"
)

func makeTransport(server, gun, tlsCaCert string) (http.RoundTripper, error) {
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

// ensureTrustDir ensures the trust directory exists
func ensureTrustDir(trustDir string) error {
	return os.MkdirAll(trustDir, 0700)
}

// clearChangelist clears the notary staging changelist
func clearChangeList(notaryRepo client.Repository) error {
	cl, err := notaryRepo.GetChangelist()
	if err != nil {
		return err
	}
	return cl.Clear("")
}

// importRootKey imports the root key from path then adds the key to repo
// returns key ids
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

// // importRootCert imports the base64 encoded public certificate corresponding to the root key
// // returns empty slice if path is empty
// func importRootCert(certFilePath string) ([]data.PublicKey, error) {
// 	publicKeys := make([]data.PublicKey, 0, 1)

// 	if certFilePath == "" {
// 		return publicKeys, nil
// 	}

// 	// read certificate from file
// 	certPEM, err := ioutil.ReadFile(certFilePath)
// 	if err != nil {
// 		return nil, fmt.Errorf("error reading certificate file: %v", err)
// 	}
// 	block, _ := pem.Decode([]byte(certPEM))
// 	if block == nil {
// 		return nil, fmt.Errorf("the provided file does not contain a valid PEM certificate %v", err)
// 	}

// 	// convert the file to data.PublicKey
// 	cert, err := x509.ParseCertificate(block.Bytes)
// 	if err != nil {
// 		return nil, fmt.Errorf("Parsing certificate PEM bytes to x509 certificate: %v", err)
// 	}
// 	publicKeys = append(publicKeys, utils.CertToKey(cert))

// 	return publicKeys, nil
// }

// Attempt to read a role key from a file, and return it as a data.PrivateKey
// If key is for the Root role, it must be encrypted
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

func getPassphraseRetriever() notary.PassRetriever {
	baseRetriever := passphrase.PromptRetriever()
	env := map[string]string{
		"root":       os.Getenv("HELM_ROOT_PASSPHRASE"),
		"targets":    os.Getenv("HELM_TARGETS_PASSPHRASE"),
		"snapshot":   os.Getenv("HELM_SNAPSHOT_PASSPHRASE"),
		"delegation": os.Getenv("HELM_DELEGATION_PASSPHRASE"),
	}

	return func(keyName string, alias string, createNew bool, numAttempts int) (string, bool, error) {
		if v := env[alias]; v != "" {
			return v, numAttempts > 1, nil
		}
		// For delegation roles, we can also try the "delegation" alias if it is specified
		// Note that we don't check if the role name is for a delegation to allow for names like "user"
		// since delegation keys can be shared across repositories
		// This cannot be a base role or imported key, though.
		if v := env["delegation"]; !data.IsBaseRole(data.RoleName(alias)) && v != "" {
			return v, numAttempts > 1, nil
		}
		return baseRetriever(keyName, alias, createNew, numAttempts)
	}
}

func getDefaultAuth() (types.AuthConfig, error) {
	cfg, err := config.Load(defaultCfgDir())
	if err != nil {
		return types.AuthConfig{}, err
	}

	return cfg.AuthConfigs[defaultIndexServer], nil
}

func defaultCfgDir() string {
	homeEnvPath := os.Getenv("HOME")
	if homeEnvPath == "" && runtime.GOOS == "windows" {
		homeEnvPath = os.Getenv("USERPROFILE")
	}

	return filepath.Join(homeEnvPath, configFileDir)
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

// GetRepoAndTag only accepts refs in the format registry/repo:tag
// TODO - Radu M
//
// rewrite this
func GetRepoAndTag(ref string) (string, string) {
	parts := strings.Split(ref, "/")
	return strings.Split(parts[1], ":")[0], strings.Split(parts[1], ":")[1]
}
