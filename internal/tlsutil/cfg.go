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
	"os"
	"path/filepath"
	"runtime"

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

func ReadCertFromSecDir(cfgFileBaseName string, host string) (opts Options, err error) {
	if runtime.GOOS == "windows" || runtime.GOOS == "unix" {
		fmt.Printf("%v OS not supported for this oci pull. Contact your administrator for more information !!!", runtime.GOOS)
	} else {
		var clientCertDir = "/etc/docker/certs.d/"
		/* var fileName = helmpath.ConfigPath(cfgFileBaseName)
		data, err := ioutil.ReadFile(fileName)

		if err != nil {
			fmt.Printf("Config file not exist %v\n", err)
		}

		var configDataMap map[string]map[string]interface{}
		err = json.Unmarshal(data, &configDataMap)

		if err != nil {
			fmt.Printf("Please do login before initiating the pull : %v", err)
		}

		keys := reflect.ValueOf(configDataMap["auths"]).MapKeys()
		strkeys := make([]string, len(keys))
		for i := 0; i < len(keys); i++ {
			strkeys[i] = keys[i].String()
		}
		//fmt.Print(strings.Join(strkeys, ","))
		strkey := strings.Join(strkeys, "") */
		clientCertDir = clientCertDir + host

		if _, err := os.Stat(clientCertDir); err != nil {
			if os.IsNotExist(err) {
				os.MkdirAll(clientCertDir, os.ModePerm)
				return opts, errors.Wrapf(err, "Client Certificate Directory Not Exist !! \n %v Directory created.", clientCertDir)
			}
		} else {

			if files, err := ioutil.ReadDir(clientCertDir); err == nil {
				for _, file := range files {
					if filepath.Ext(file.Name()) == ".crt" {
						opts.CaCertFile = clientCertDir + "/" + file.Name()
						//fmt.Printf("cafile: %v\n", opts.CaCertFile)
					} else if filepath.Ext(file.Name()) == ".pem" {
						opts.CertFile = clientCertDir + "/" + file.Name()
						//fmt.Printf("certFile: %v\n", opts.CertFile)
					} else if filepath.Ext(file.Name()) == ".key" {
						opts.KeyFile = clientCertDir + "/" + file.Name()
						//fmt.Printf("keyFile: %v\n", opts.KeyFile)
					}
				}
			} else {
				fmt.Printf(" Certificate not found in current directory - %v\n ", err)
				os.Exit(1)
			}
			if opts.CaCertFile == "" && opts.CertFile == "" && opts.KeyFile == "" {
				fmt.Printf("Error Certificate (cacerts.crt,client.pem,client.key) required : Client authentication failed due to certificate not present in cert directory !! \n")
				os.Exit(1)
			}

			if opts.CaCertFile == "" && opts.CertFile == "" {
				fmt.Printf("Error Certificate Required : Root-CA and client certificate (cacerts.crt,client.pem) not found.\n")
				os.Exit(1)
			}

			if opts.CaCertFile == "" && opts.KeyFile == "" {
				fmt.Printf("Error Certificate Required :  Root-CA and and client keyfie (cacerts.crt,client.key) not found.\n")
				os.Exit(1)
			}

			if opts.CertFile == "" && opts.KeyFile == "" {
				fmt.Printf("Error Certificate Required : Client certificate and client keyfile (client.pem,client.key) not found.\n")
				os.Exit(1)
			}
			if opts.CaCertFile == "" {
				fmt.Printf("Error Certificate Required : Client Root-CA (cacerts.crt) not found.\n")
				os.Exit(1)
			} else if opts.CertFile == "" {
				fmt.Printf("Error Certificate Required : Client certificate(client.pem) not found.\n")
				os.Exit(1)
			} else if opts.KeyFile == "" {
				fmt.Printf("Error Certificate Required : Client keyfile (client.key) not found.\n")
				os.Exit(1)
			}
		}
	}
	return opts, nil
}
