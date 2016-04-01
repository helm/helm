// build +e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const defaultKubectlPath = "kubectl"

type KubeContext struct {
	Path   string
	Config *Config
}

func NewKubeContext() *KubeContext {
	return &KubeContext{
		Path: defaultKubectlPath,
	}
}

type Config struct {
	Clusters []struct {
		// Name is the nickname for this Cluster
		Name string `json:"name"`
		// Cluster holds the cluster information
		Cluster struct {
			// Server is the address of the kubernetes cluster (https://hostname:port).
			Server string `json:"server"`
			// APIVersion is the preferred api version for communicating with the kubernetes cluster (v1, v2, etc).
			APIVersion string `json:"api-version,omitempty"`
			// InsecureSkipTLSVerify skips the validity check for the server's certificate. This will make your HTTPS connections insecure.
			InsecureSkipTLSVerify bool `json:"insecure-skip-tls-verify,omitempty"`
			// CertificateAuthority is the path to a cert file for the certificate authority.
			CertificateAuthority string `json:"certificate-authority,omitempty"`
			// CertificateAuthorityData contains PEM-encoded certificate authority certificates. Overrides CertificateAuthority
			CertificateAuthorityData []byte `json:"certificate-authority-data,omitempty"`
			// Extensions holds additional information. This is useful for extenders so that reads and writes don't clobber unknown fields
			Extensions []struct {
				// Name is the nickname for this Extension
				Name string `json:"name"`
			} `json:"extensions,omitempty"`
		} `json:"cluster"`
	}
}

func (k *KubeContext) ParseConfig() {
	out, _ := exec.Command(k.Path, "config", "view", "--flatten=true", "--minify=true", "-o=json").Output()
	err := json.Unmarshal(out, &k.Config)
	if err != nil {
		fmt.Println(err.Error())
	}
}

func (k *KubeContext) Cluster() string {
	out, _ := exec.Command(k.Path, "config", "view", "--flatten=true", "--minify=true", "-o", "jsonpath='{.clusters[0].name}'").Output()
	return string(out)
}

func (k *KubeContext) Server() string {
	out, _ := exec.Command(k.Path, "config", "view", "--flatten=true", "--minify=true", "-o", "jsonpath='{.clusters[0].cluster.server}'").Output()
	return strings.Replace(string(out), "'", "", -1)
}

func (k *KubeContext) CurrentContext() string {
	out, _ := exec.Command(k.Path, "config", "view", "--flatten=true", "--minify=true", "-o", "jsonpath='{.current-context}'").Output()
	return string(out)
}

func (k *KubeContext) Running() bool {
	_, err := exec.Command(k.Path, "cluster-info").CombinedOutput()
	return err == nil
}

func (k *KubeContext) Version() string {
	out, _ := exec.Command(k.Path, "version").Output()
	return string(out)
}
