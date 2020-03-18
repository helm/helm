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

package main

import (
	"encoding/hex"
	"fmt"
	"github.com/theupdateframework/notary/client"
	"github.com/theupdateframework/notary/trustpinning"
	"github.com/theupdateframework/notary/tuf/data"
	"helm.sh/helm/v3/internal/experimental/registry"
	"helm.sh/helm/v3/pkg/helmpath"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/action"
)

const chartPullDesc = `
Download a chart from a remote registry.

This will store the chart in the local registry cache to be used later.
`

func newChartPullCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	signOpts := &signatureOptions{}
	cmd := &cobra.Command{
		Use:    "pull [ref]",
		Short:  "pull a chart from remote",
		Long:   chartPullDesc,
		Args:   require.MinimumNArgs(1),
		Hidden: !FeatureGateOCI.IsEnabled(),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			err := action.NewChartPull(cfg).Run(out, ref)
			if err != nil {
				return err
			}

			if signOpts.Sign {
				sha, err := GetSHA(signOpts.trustDir, signOpts.trustServer, ref, signOpts.caCert, signOpts.rootKey)
				if err != nil {
					return err
				}

				r, err := registry.ParseReference(ref)
				if err != nil {
					return err
				}

				c, err := registry.NewCache(
					registry.CacheOptWriter(out),
					registry.CacheOptRoot(filepath.Join(helmpath.CachePath(), "registry", registry.CacheRootDir)))

				cs, err := c.FetchReference(r)
				if err != nil {
					return err
				}

				if cs.Digest.Hex() != sha {
					fmt.Fprintf(out, "digests do not match: %v and %v", cs.Digest.Hex(), sha)
					_, err = c.DeleteReference(r)
					return err
				}
			}
			return nil
		},
	}
	td := filepath.Join(helmpath.ConfigPath(), ".trust")
	cmd.Flags().StringVarP(&signOpts.trustServer, "trust-server", "", "", "The trust server to use for signature verification")
	cmd.Flags().StringVarP(&signOpts.trustDir, "trust-dir", "", td, "Location where trust data is stored")
	cmd.Flags().StringVarP(&signOpts.rootKey, "root-key", "", "", "Root Key to initialize repository with")
	cmd.Flags().StringVarP(&signOpts.caCert, "ca-cert", "", "", "Trust certs signed only by this CA will be considered")
	cmd.Flags().BoolVarP(&signOpts.Sign, "sign", "", true, "Enable signature checking")

	return cmd
}

func GetSHA(trustDir, trustServer, ref, tlscacert, rootKey string) (string, error) {
	r, tag := GetRepoAndTag(ref)
	target, err := GetTargetWithRole(r, tag, trustServer, tlscacert, trustDir)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(target.Hashes["sha256"]), nil
}

func GetRepoAndTag(ref string) (string, string) {
	parts := strings.Split(ref, "/")
	return strings.Split(parts[1], ":")[0], strings.Split(parts[1], ":")[1]
}

func GetTargetWithRole(gun, name, trustServer, tlscacert, trustDir string) (*client.TargetWithRole, error) {
	targets, err := GetTargets(gun, trustServer, tlscacert, trustDir)
	if err != nil {
		return nil, fmt.Errorf("cannot list targets:%v", err)
	}

	for _, target := range targets {
		if target.Name == name {
			return target, nil
		}
	}

	return nil, fmt.Errorf("cannot find target %v in trusted collection %v", name, gun)
}

// GetTargets returns all targets for a given gun from the trusted collection
func GetTargets(gun, trustServer, tlscacert, trustDir string) ([]*client.TargetWithRole, error) {
	if err := ensureTrustDir(trustDir); err != nil {
		return nil, fmt.Errorf("cannot ensure trust directory: %v", err)
	}

	transport, err := action.MakeTransport(trustServer, gun, tlscacert)
	if err != nil {
		return nil, fmt.Errorf("cannot make transport: %v", err)
	}

	repo, err := client.NewFileCachedRepository(
		trustDir,
		data.GUN(gun),
		trustServer,
		transport,
		nil,
		trustpinning.TrustPinConfig{},
	)
	if err != nil {
		return nil, fmt.Errorf("cannot create new file cached repository: %v", err)
	}

	return repo.ListTargets()
}

// ensureTrustDir ensures the trust directory exists
func ensureTrustDir(trustDir string) error {
	return os.MkdirAll(trustDir, 0700)
}