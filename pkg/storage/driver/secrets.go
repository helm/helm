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

package driver // import "helm.sh/helm/v3/pkg/storage/driver"

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kblabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	rspb "helm.sh/helm/v3/pkg/release"
)

var _ Driver = (*Secrets)(nil)

const SizeCutoff = 1048576 // https://kubernetes.io/docs/concepts/configuration/secret/#restriction-data-size
const HelmPartialStorageType = "sh.helm.partial.v1"

// SecretsDriverName is the string name of the driver.
const SecretsDriverName = "Secret"

// Secrets is a wrapper around an implementation of a kubernetes
// SecretsInterface.
type Secrets struct {
	impl corev1.SecretInterface
	Log  func(string, ...interface{})
}

// The "partial" pendant to pkg.storage.driver.storage:makeKey
func makePartialKey(rlsname string, version int, chunkIndex int) string {
	return fmt.Sprintf("%s.%s.v%d-%d", HelmPartialStorageType, rlsname, version, chunkIndex)
}

// NewSecrets initializes a new Secrets wrapping an implementation of
// the kubernetes SecretsInterface.
func NewSecrets(impl corev1.SecretInterface) *Secrets {
	return &Secrets{
		impl: impl,
		Log:  func(_ string, _ ...interface{}) {},
	}
}

// Name returns the name of the driver.
func (secrets *Secrets) Name() string {
	return SecretsDriverName
}

// _FetchReleaseData is an internal function to fetch the release data
// from the release secret and subsequent partial secrets.
func (secrets *Secrets) _FetchReleaseData(first *v1.Secret) (string, error) {
	data := string(first.Data["release"])
	nextKey, ok := first.Labels["continuedIn"]
	for ok {
		obj, err := secrets.impl.Get(context.Background(), nextKey, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return "", errors.Wrapf(ErrReleaseNotFound, "partial release not found %q", nextKey)
			}
			return "", errors.Wrapf(err, "failed to get partial %q", nextKey)
		}
		data = data + string(obj.Data["release"])
		nextKey, ok = obj.Labels["continuedIn"]
	}
	return data, nil
}

// Get fetches the release named by key. The corresponding release is returned
// or error if not found.
func (secrets *Secrets) Get(key string) (*rspb.Release, error) {
	// fetch the secret holding the release named by key
	obj, err := secrets.impl.Get(context.Background(), key, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, ErrReleaseNotFound
		}
		return nil, errors.Wrapf(err, "get: failed to get %q", key)
	}
	data, err := secrets._FetchReleaseData(obj)
	if err != nil {
		return nil, errors.Wrapf(err, "get: failed to fetch release data %q", key)
	}
	// decode the base64 data string
	r, err := decodeRelease(data)
	return r, errors.Wrapf(err, "get: failed to decode data %q", key)
}

// List fetches all releases and returns the list releases such
// that filter(release) == true. An error is returned if the
// secret fails to retrieve the releases.
func (secrets *Secrets) List(filter func(*rspb.Release) bool) ([]*rspb.Release, error) {
	lsel := kblabels.Set{"owner": "helm"}.AsSelector()
	opts := metav1.ListOptions{LabelSelector: lsel.String()}

	list, err := secrets.impl.List(context.Background(), opts)
	if err != nil {
		return nil, errors.Wrap(err, "list: failed to list")
	}

	var results []*rspb.Release

	// iterate over the secrets object list
	// and decode each release
	for _, item := range list.Items {
		data, err := secrets._FetchReleaseData(&item)
		if err != nil {
			secrets.Log("list: failed to fetch release data: %v: %s", item, err)
			continue
		}
		// decode the base64 data string
		rls, err := decodeRelease(data)
		if err != nil {
			secrets.Log("list: failed to decode release: %v: %s", item, err)
			continue
		}

		rls.Labels = item.ObjectMeta.Labels

		if filter(rls) {
			results = append(results, rls)
		}
	}
	return results, nil
}

// Query fetches all releases that match the provided map of labels.
// An error is returned if the secret fails to retrieve the releases.
func (secrets *Secrets) Query(labels map[string]string) ([]*rspb.Release, error) {
	ls := kblabels.Set{}
	for k, v := range labels {
		if errs := validation.IsValidLabelValue(v); len(errs) != 0 {
			return nil, errors.Errorf("invalid label value: %q: %s", v, strings.Join(errs, "; "))
		}
		ls[k] = v
	}

	opts := metav1.ListOptions{LabelSelector: ls.AsSelector().String()}

	list, err := secrets.impl.List(context.Background(), opts)
	if err != nil {
		return nil, errors.Wrap(err, "query: failed to query with labels")
	}

	if len(list.Items) == 0 {
		return nil, ErrReleaseNotFound
	}

	var results []*rspb.Release
	for _, item := range list.Items {
		data, err := secrets._FetchReleaseData(&item)
		if err != nil {
			secrets.Log("query: failed to fetch release data: %s", err)
			continue
		}
		rls, err := decodeRelease(data)
		if err != nil {
			secrets.Log("query: failed to decode release: %s", err)
			continue
		}
		results = append(results, rls)
	}
	return results, nil
}

// Create creates a new Secret holding the release. If the
// Secret already exists, ErrReleaseExists is returned.
func (secrets *Secrets) Create(key string, rls *rspb.Release) error {
	// set labels for secrets object meta data
	var lbs labels

	lbs.init()
	lbs.set("createdAt", strconv.Itoa(int(time.Now().Unix())))

	// create a new secret to hold the release
	secretsList, err := newSecretObjects(key, rls, lbs)
	if err != nil {
		return errors.Wrapf(err, "create: failed to encode release %q", rls.Name)
	}
	// push the secret objects out into the kubiverse
	for _, obj := range secretsList {
		if _, err := secrets.impl.Create(context.Background(), obj, metav1.CreateOptions{}); err != nil {
			if apierrors.IsAlreadyExists(err) {
				return errors.Wrapf(ErrReleaseExists, "create: key %s already exists", obj.ObjectMeta.Name)
			}
			return errors.Wrap(err, "create: failed to create")
		}
	}
	return nil
}

// Update updates the Secret holding the release. If not found
// the Secret is created to hold the release.
func (secrets *Secrets) Update(key string, rls *rspb.Release) error {
	// get current release secret
	obj, err := secrets.impl.Get(context.Background(), key, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ErrReleaseNotFound
		}
		return errors.Wrapf(err, "update: failed to get %q", key)
	}

	partialKeys := map[string]bool{} // store if this partial should be deleted
	partialKeys[key] = false         // add the first release key, never delete it

	// get keys for existing partial items if there's any
	// don't use _FetchReleaseData as we only need the keys, not the data
	nextKey, ok := obj.Labels["continuedIn"]
	for ok {
		partialKeys[nextKey] = true
		obj, err := secrets.impl.Get(context.Background(), nextKey, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return errors.Wrapf(ErrReleaseNotFound, "update: partial release not found %q", nextKey)
			}
			return errors.Wrapf(err, "update: failed to get %q", key)
		}
		nextKey, ok = obj.Labels["continuedIn"]
	}

	// set labels for secrets object meta data
	var lbs labels

	lbs.init()
	lbs.set("modifiedAt", strconv.Itoa(int(time.Now().Unix())))

	// create new secret objects to hold the updated release
	secretsList, err := newSecretObjects(key, rls, lbs)
	if err != nil {
		return errors.Wrapf(err, "update: failed to encode release %q", rls.Name)
	}

	// update secrets as needed
	for _, obj := range secretsList {
		_, ok = partialKeys[obj.ObjectMeta.Name]
		if ok {
			partialKeys[obj.ObjectMeta.Name] = false
			if _, err := secrets.impl.Update(context.Background(), obj, metav1.UpdateOptions{}); err != nil {
				return errors.Wrap(err, "update: failed to update")
			}
		} else {
			if _, err := secrets.impl.Create(context.Background(), obj, metav1.CreateOptions{}); err != nil {
				return errors.Wrap(err, "update: failed to create new partial")
			}
		}
	}
	// delete any extra partials
	for key, shouldRemove := range partialKeys {
		if shouldRemove {
			if err := secrets.impl.Delete(context.Background(), key, metav1.DeleteOptions{}); err != nil {
				return errors.Wrap(err, "update: failed to delete extra partial")
			}
		}
	}
	return nil
}

// Delete deletes the Secret holding the release named by key.
func (secrets *Secrets) Delete(key string) (rls *rspb.Release, err error) {
	// fetch the release to check existence
	if rls, err = secrets.Get(key); err != nil {
		return nil, err
	}

	// fetch main release object
	obj, err := secrets.impl.Get(context.Background(), key, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "delete: failed to get release %q", key)
	}

	// don't use _FetchReleaseData as we only need the keys, not the data
	// fetch all keys that need to be deleted
	var keys []string

	nextKey, ok := obj.Labels["continuedIn"]
	for ok {
		obj, err := secrets.impl.Get(context.Background(), nextKey, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, errors.Wrapf(ErrReleaseNotFound, "delete: partial release not found %q", nextKey)
			}
			return nil, errors.Wrapf(err, "delete: failed to get partial %q", nextKey)
		}
		// Add the partial key to the list of partial keys to delete
		keys = append(keys, nextKey)
		// Prepare next iteration
		nextKey, ok = obj.Labels["continuedIn"]
	}
	// delete all objects
	if err := secrets.impl.Delete(context.Background(), key, metav1.DeleteOptions{}); err != nil {
		return rls, errors.Wrapf(err, "delete: failed to delete %q", key)
	}
	for _, deleteKey := range keys {
		if err := secrets.impl.Delete(context.Background(), deleteKey, metav1.DeleteOptions{}); err != nil {
			return rls, errors.Wrapf(err, "delete: failed to delete partial %q", deleteKey)
		}
	}
	return rls, nil
}

// newSecretObjects constructs an array of kubernetes Secret objects
// to store a release.
// The data stored within these secrets is the base64 encoded, gzipped string
// of a release, split across multiple secrets if needed.
// The maximum size of a secret, when this code was written, is 1Mib, as defined here
// https://kubernetes.io/docs/concepts/configuration/secret/#restriction-data-size
//
// The following labels are used within each secret:
//
//	"modifiedAt"     - timestamp indicating when this secret was last modified. (set in Update)
//	"createdAt"      - timestamp indicating when this secret was created. (set in Create)
//	"version"        - version of the release.
//	"status"         - status of the release (see pkg/release/status.go for variants)
//	"owner"          - owner of the secret, currently "helm".
//	"name"           - name of the release.
//	"continuedIn"    - if set, the encoded contents of the release continue in the secret this references.
func newSecretObjects(key string, rls *rspb.Release, lbs labels) ([]*v1.Secret, error) {
	const owner = "helm"

	// encode the release
	s, err := encodeRelease(rls)
	if err != nil {
		return nil, err
	}

	if lbs == nil {
		lbs.init()
	}

	releaseBytes := []byte(s)

	// apply labels
	lbs.set("name", rls.Name)
	lbs.set("owner", owner)
	lbs.set("status", rls.Info.Status.String())
	lbs.set("version", strconv.Itoa(rls.Version))

	// Helm 3 introduced setting the 'Type' field
	// in the Kubernetes storage object.
	// Helm defines the field content as follows:
	// <helm_domain>/<helm_object>.v<helm_object_version>
	// Type field for Helm 3: helm.sh/release.v1
	// Note: Version starts at 'v1' for Helm 3 and
	// should be incremented if the release object
	// metadata is modified.
	// This would potentially be a breaking change
	// and should only happen between major versions.

	var secrets []*v1.Secret

	if len(releaseBytes) <= SizeCutoff {
		// early return to only create the first object
		return append(secrets, &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:   key,
				Labels: lbs.toMap(),
			},
			Type: "helm.sh/release.v1",
			Data: map[string][]byte{"release": releaseBytes},
		}), nil
	}

	// create copy of the labels
	var currentLabels labels
	currentLabels.init()
	currentLabels.fromMap(lbs.toMap())

	// create a secret with the first chunk of data
	firstSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   key,
			Labels: currentLabels.toMap(),
		},
		Type: "helm.sh/release.v1",
		Data: map[string][]byte{"release": releaseBytes[0:SizeCutoff]},
	}

	// build the reference to the next chunk
	var currentChunkIndex = 1
	currentChunkKey := makePartialKey(rls.Name, rls.Version, currentChunkIndex)

	// add the continuedIn field
	firstSecret.ObjectMeta.Labels["continuedIn"] = currentChunkKey

	// append to the list of secrets to create
	secrets = append(secrets, firstSecret)

	// prepare to split
	// use a window defined by idxStart:idxStop
	var idxStart int
	var idxStop = SizeCutoff
	for idxStop != len(releaseBytes) {
		// shift window
		idxStart += SizeCutoff
		idxStop += SizeCutoff
		// don't overread - cap idxStop
		if idxStop > len(releaseBytes) {
			idxStop = len(releaseBytes)
		}

		currentLabels.init()
		currentLabels.fromMap(lbs.toMap())
		// create secret to store partial data
		currentSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:   currentChunkKey, // this is the key the previous chunk will point to
				Labels: currentLabels.toMap(),
			},
			Type: "helm.sh/partial.v1", // custom type to indicate this isn't a release definition in itself
			Data: map[string][]byte{"release": releaseBytes[idxStart:idxStop]},
		}
		// check if we'll need another partial chunk
		if idxStop != len(releaseBytes) {
			currentChunkIndex++                                                        // increment current chunk
			currentChunkKey = makePartialKey(rls.Name, rls.Version, currentChunkIndex) // make key for the next chunk
			currentSecret.ObjectMeta.Labels["continuedIn"] = currentChunkKey           // store reference to it
		}
		secrets = append(secrets, currentSecret)
	}

	return secrets, nil
}
