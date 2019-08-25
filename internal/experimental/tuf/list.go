package tuf

import (
	"encoding/hex"
	"fmt"

	"github.com/theupdateframework/notary/client"
	"github.com/theupdateframework/notary/trustpinning"
	"github.com/theupdateframework/notary/tuf/data"
)

// PrintTargets prints all the targets for a specific GUN from a trust server
func PrintTargets(gun, trustServer, tlscacert, trustDir string) error {
	targets, err := GetTargets(gun, trustServer, tlscacert, trustDir)
	if err != nil {
		return fmt.Errorf("cannot list targets:%v", err)
	}

	for _, tgt := range targets {
		fmt.Printf("%s\t%s\n", tgt.Name, hex.EncodeToString(tgt.Hashes["sha256"]))
	}
	return nil
}

// GetTargetWithRole returns a single target by name from the trusted collection
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

	transport, err := makeTransport(trustServer, gun, tlscacert)
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

// GetSHA returns the digest stored in a trust server for a given reference
func GetSHA(trustDir, trustServer, ref, tlscacert, rootKey string) (string, error) {
	r, tag := GetRepoAndTag(ref)
	target, err := GetTargetWithRole(r, tag, trustServer, tlscacert, trustDir)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(target.Hashes["sha256"]), nil
}
