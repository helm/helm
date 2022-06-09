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

package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"
)

// Options represents configurable options used to create client and server TLS configurations.
type Options struct {
	CaCertFile string
	// If either the KeyFile or CertFile is empty, ClientConfig() will not load them.
	KeyFile  string
	CertFile string
	// Client-only options
	InsecureSkipVerify bool
}

// ClientConfig returns a TLS configuration for use by a Helm client.
func ClientConfig(opts Options) (cfg *tls.Config, err error) {
	var cert *tls.Certificate
	var pool *x509.CertPool

	if opts.CertFile != "" || opts.KeyFile != "" {
		if cert, err = CertFromFilePair(opts.CertFile, opts.KeyFile); err != nil {
			if os.IsNotExist(err) {
				return nil, errors.Wrapf(err, "could not load x509 key pair (cert: %q, key: %q)", opts.CertFile, opts.KeyFile)
			}
			return nil, errors.Wrapf(err, "could not read x509 key pair (cert: %q, key: %q)", opts.CertFile, opts.KeyFile)
		}
	}
	if !opts.InsecureSkipVerify && opts.CaCertFile != "" {
		if pool, err = CertPoolFromFile(opts.CaCertFile); err != nil {
			return nil, err
		}
	}

	cfg = &tls.Config{InsecureSkipVerify: opts.InsecureSkipVerify, Certificates: []tls.Certificate{*cert}, RootCAs: pool}
	return cfg, nil
}

func ReadCertFromSecDir(host string) (opts Options, err error) {
	//fmt.Println("Final Host Name : ", host)
	if runtime.GOOS == "windows" || runtime.GOOS == "unix" {
		log.Fatalf("%v OS not supported for this oci pull.", runtime.GOOS)
		os.Exit(1)
	} else {
		cmd, err := exec.Command("helm", "env", "HELM_CLIENT_TLS_CERT_DIR").Output()
		if err != nil {
			log.Fatalf("Error : %s", err)
			os.Exit(1)
		}
		clientCertDir := strings.TrimSuffix(string(cmd), "\n")
		if clientCertDir == "" {
			log.Fatalf("Please configure client certificate directory for tls connection set/export HELM_CLIENT_TLS_CERT_DIR='/etc/docker/certs.d/'\n")
			os.Exit(1)
		}

		if clientCertDir[len(clientCertDir)-1] != '/' {
			clientCertDir = fmt.Sprintf("%s/%s", clientCertDir, host)
			//fmt.Println("clientCertDir1", clientCertDir)
		} else {
			clientCertDir = fmt.Sprintf("%s%s", clientCertDir, host)
			//fmt.Println("clientCertDir2", clientCertDir)
		}
		if _, err := os.Stat(clientCertDir); err != nil {
			if os.IsNotExist(err) {
				return opts, errors.Wrapf(err, clientCertDir, "%v\nPlease Create a directory same as hostname [%v] .")
			}
		} else {

			if files, err := ioutil.ReadDir(clientCertDir); err == nil {
				for _, file := range files {
					if filepath.Ext(file.Name()) == ".pem" {
						opts.CaCertFile = fmt.Sprintf("%s/%s", clientCertDir, file.Name())
						//fmt.Println("Root ca file : ", opts.CaCertFile)
					} else if filepath.Ext(file.Name()) == ".cert" {
						opts.CertFile = fmt.Sprintf("%s/%s", clientCertDir, file.Name())
						//fmt.Println("client cert file : ", opts.CertFile)
					} else if filepath.Ext(file.Name()) == ".key" {
						opts.KeyFile = fmt.Sprintf("%s/%s", clientCertDir, file.Name())
						//fmt.Println("client key file", opts.KeyFile)
					}
				}
			} else {
				log.Fatalf(" Certificate not found in current directory - %v\n ", err)
				os.Exit(1)
			}
			switch {
			case opts.CaCertFile == "" && opts.CertFile == "" && opts.KeyFile == "":
				fmt.Printf("Error : Missing certificate (cacerts.crt,client.pem,client.key) required !!\n")
				os.Exit(1)
			case opts.CaCertFile == "" && opts.CertFile == "":
				fmt.Printf("Error : Missing certificate : Root-CA and client certificate (cacerts.crt,client.pem) required !!\n")
				os.Exit(1)
			case opts.CaCertFile == "" && opts.KeyFile == "":
				fmt.Printf("Error : Missing Certificate : Root-CA and and client key (cacerts.crt,client.key) required.\n")
				os.Exit(1)

			case opts.CertFile == "" && opts.KeyFile == "":
				fmt.Printf("Error : Missing Certificate : Client certificate and client key (client.pem,client.key) required.\n")
				os.Exit(1)
			}
			switch {
			case opts.CaCertFile == "":
				fmt.Printf("Error : Missing Certificate : Client Root-CA (cacerts.crt) required.\n")
				os.Exit(1)
			case opts.CertFile == "":
				fmt.Printf("Error : Missing Certificate : Client certificate(client.pem) required.\n")
				os.Exit(1)
			case opts.KeyFile == "":
				fmt.Printf("Error : Missing Certificate : Client keyfile (client.key) required.\n")
				os.Exit(1)

			}
		}
	}
	return opts, nil
}
