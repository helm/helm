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

// Supports both single and multiple Secrets to hold a Helm release.

// TODO
// Arch
//   - Consider implementing chunk support as v2 driver?
// Code
//   - Complete the remove code duplication of chunking code
//     See loadRemainingChunks() not currently working b/c no access to secrets.impl.Get()
//   - Likely remove HELM_DRIVER_CHUNKSIZE setting and fix size at ~1MB
//   - Tests

package driver // import "helm.sh/helm/v3/pkg/storage/driver"

import (
	"context"
	"fmt"
	"os"
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

var _ Driver = (*MultiSecrets)(nil)

// MultiSecretsDriverName is the string name of the driver.
const MultiSecretsDriverName = "MultiSecret"

// MultiSecrets is a wrapper around an implementation of a kubernetes
// SecretsInterface.
type MultiSecrets struct {
	impl corev1.SecretInterface
	Log  func(string, ...interface{})
	//	loadRemainingChunks func(string, ...interface{}) ([]byte, error)
}

// NewMultiSecrets initializes a new Secrets wrapping an implementation of
// the kubernetes SecretsInterface.
func NewMultiSecrets(impl corev1.SecretInterface) *MultiSecrets {
	return &MultiSecrets{
		impl: impl,
		Log:  func(_ string, _ ...interface{}) {},
		//		loadRemainingChunks: func(_ string, _ ...interface{}) (_ []byte, _ error) {},
	}
}

// Name returns the name of the driver.
func (secrets *MultiSecrets) Name() string {
	return MultiSecretsDriverName
}

// Get fetches the release named by key. The corresponding release is returned
// or error if not found.
func (secrets *MultiSecrets) Get(key string) (*rspb.Release, error) {
	// fetch the secret holding the release named by key
	obj, err := secrets.impl.Get(context.Background(), key, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, ErrReleaseNotFound
		}
		return nil, errors.Wrapf(err, "get: failed to get %q", key)
	}

	// Add remaining chunks; use single function
	// obj.Data["release"], _ = secrets.loadRemainingChunks(key, obj)
	// Let decode release fail if release contains incorrect data?

	// Add remaining chunks; duplicated code

	chunks, _ := strconv.Atoi(string(obj.Data["chunks"]))
	for chunk := 2; chunk <= chunks; chunk++ {
		key := fmt.Sprintf("%s.%d", obj.ObjectMeta.Name, chunk)
		chunkobj, _ := secrets.impl.Get(context.Background(), key, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, ErrReleaseNotFound
			}
			return nil, errors.Wrapf(err, "get: failed to get %q", key)
		}
		obj.Data["release"] = append(obj.Data["release"], chunkobj.Data["release"]...)
	}

	// found the secret, decode the base64 data string
	r, err := decodeRelease(string(obj.Data["release"]))
	return r, errors.Wrapf(err, "get: failed to decode data %q", key)
}

// List fetches all releases and returns the list releases such
// that filter(release) == true. An error is returned if the
// secret fails to retrieve the releases.
func (secrets *MultiSecrets) List(filter func(*rspb.Release) bool) ([]*rspb.Release, error) {
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
		// If chunked, add remaining chunks
		chunk, err := strconv.Atoi(string(item.Data["chunk"]))
		if err == nil && chunk > 1 {
			continue
		} else {
			chunks, _ := strconv.Atoi(string(item.Data["chunks"]))
			for chunk := 2; chunk <= chunks; chunk++ {
				key := fmt.Sprintf("%s.%d", item.ObjectMeta.Name, chunk)
				chunkobj, _ := secrets.impl.Get(context.Background(), key, metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						return nil, ErrReleaseNotFound
					}
					return nil, errors.Wrapf(err, "get: failed to get %q", key)
				}
				item.Data["release"] = append(item.Data["release"], chunkobj.Data["release"]...)
			}
		}

		rls, err := decodeRelease(string(item.Data["release"]))
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
func (secrets *MultiSecrets) Query(labels map[string]string) ([]*rspb.Release, error) {
	const firstchunk = 1
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
		// If chunked, add remaining chunks
		chunks, _ := strconv.Atoi(string(item.Data["chunks"]))
		for chunk := 2; chunk <= chunks; chunk++ {
			key := fmt.Sprintf("%s.%d", item.ObjectMeta.Name, chunk)
			chunkobj, _ := secrets.impl.Get(context.Background(), key, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil, ErrReleaseNotFound
				}
				return nil, errors.Wrapf(err, "get: failed to get %q", key)
			}
			item.Data["release"] = append(item.Data["release"], chunkobj.Data["release"]...)
		}

		rls, err := decodeRelease(string(item.Data["release"]))
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
func (secrets *MultiSecrets) Create(key string, rls *rspb.Release) error {
	// set labels for secrets object meta data
	var lbs labels

	lbs.init()
	lbs.set("createdAt", strconv.Itoa(int(time.Now().Unix())))

	// create a new secret to hold the release
	objs, err := newMultiSecretsObject(key, rls, lbs, 1)
	if err != nil {
		return errors.Wrapf(err, "create: failed to encode release %q", rls.Name)
	}
	// push the secret object out into the kubiverse
	for _, obj := range *objs {
		if _, err := secrets.impl.Create(context.Background(), &obj, metav1.CreateOptions{}); err != nil {
			if apierrors.IsAlreadyExists(err) {
				return ErrReleaseExists
			}

			return errors.Wrap(err, "create: failed to create")
		}
	}
	return nil
}

// Update updates the Secret holding the release. If not found
// the Secret is created to hold the release.
func (secrets *MultiSecrets) Update(key string, rls *rspb.Release) error {
	// Get release 1st to check if chunked or not
	obj, err := secrets.impl.Get(context.Background(), key, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "get: release not found %q", rls.Name)
		}
		return errors.Wrapf(err, "get: failed to get %q", key)
	}
	chunks, err := strconv.Atoi(string(obj.Data["chunks"]))
	if err != nil {
		chunks = 0
	}

	// set labels for secrets object meta data
	var lbs labels

	lbs.init()
	lbs.set("modifiedAt", strconv.Itoa(int(time.Now().Unix())))

	// create a new secret object to hold the release
	objs, err := newMultiSecretsObject(key, rls, lbs, chunks)
	if err != nil {
		return errors.Wrapf(err, "update: failed to encode release %q", rls.Name)
	}
	// push the secret object out into the kubiverse
	for _, obj := range *objs {
		if _, err = secrets.impl.Update(context.Background(), &obj, metav1.UpdateOptions{}); err != nil {
			return errors.Wrap(err, "update: failed to update")
		}
	}
	return nil
}

// Delete deletes the Secret holding the release named by key.
func (secrets *MultiSecrets) Delete(key string) (rls *rspb.Release, err error) {
	// fetch the secret holding the release named by key
	obj, err := secrets.impl.Get(context.Background(), key, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, ErrReleaseNotFound
		}
		return nil, errors.Wrapf(err, "get: failed to get %q", key)
	}
	chunks, err := strconv.Atoi(string(obj.Data["chunks"]))
	if err != nil {
		chunks = 0
	}

	// fetch the release to check existence
	if rls, err = secrets.Get(key); err != nil {
		return nil, err
	}

	// delete the release
	err = secrets.impl.Delete(context.Background(), key, metav1.DeleteOptions{})
	// delete remaining chunks if any
	for chunk := 2; chunk <= chunks; chunk++ {
		err = secrets.impl.Delete(context.Background(), fmt.Sprintf("%s.%d", key, chunk), metav1.DeleteOptions{})
	}
	return rls, err
}

// newSecretsObject constructs a kubernetes Secret object
// to store a release. Each secret data entry is the base64
// encoded gzipped string of a release.
//
// The following labels are used within each secret:
//
//	"modifiedAt"    - timestamp indicating when this secret was last modified. (set in Update)
//	"createdAt"     - timestamp indicating when this secret was created. (set in Create)
//	"version"        - version of the release.
//	"status"         - status of the release (see pkg/release/status.go for variants)
//	"owner"          - owner of the secret, currently "helm".
//	"name"           - name of the release.
func newMultiSecretsObject(key string, rls *rspb.Release, lbs labels, chunks int) (*[]v1.Secret, error) {
	const owner = "helm"
	const maxChunkSize = (1024 * 1000) // approx 1MB in bytes

	// encode the release
	s, err := encodeRelease(rls)
	if err != nil {
		return nil, err
	}

	if lbs == nil {
		lbs.init()
	}

	// apply labels
	lbs.set("name", rls.Name)
	lbs.set("owner", owner)
	lbs.set("status", rls.Info.Status.String())
	lbs.set("version", strconv.Itoa(rls.Version))

	// create and return secret object.
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

	objs := []v1.Secret{}
	if chunks > 0 {
		origData := s

		chunkSize := maxChunkSize
		sz := strings.TrimSpace(os.Getenv("HELM_DRIVER_CHUNKSIZE"))
		if sz != "" {
			size, err := strconv.Atoi(sz)
			if err == nil && size < maxChunkSize {
				chunkSize = size
			} else {
				return nil, errors.Wrapf(err, "newSecretsObject: cannot use chunk size: %s", sz)
			}
		}
		lbs.set("chunksize", strconv.Itoa(chunkSize))

		slices := []string{}
		lastIndex := 0
		lastI := 0
		for i := range origData {
			if i-lastIndex > chunkSize {
				slices = append(slices, origData[lastIndex:lastI])
				lastIndex = lastI
			}
			lastI = i
		}
		// handle the leftovers at the end
		if len(origData)-lastIndex > chunkSize {
			slices = append(slices, origData[lastIndex:lastIndex+chunkSize], origData[lastIndex+chunkSize:])
		} else {
			slices = append(slices, origData[lastIndex:])
		}

		i := 1
		for _, str := range slices {
			{
				var lbs2 labels
				lbs2.init()
				lbs2.set("chunk", strconv.Itoa(i))
				lbs2.set("chunks", strconv.Itoa(len(slices)))
				instanceName := key
				if i > 1 {
					instanceName = fmt.Sprintf("%s.%d", key, i)
				}
				objs = append(objs, v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:   instanceName,
						Labels: kblabels.Merge(lbs.toMap(), lbs2.toMap()),
					},
					Type: "helm.sh/release.v1",
					Data: map[string][]byte{"release": []byte(str), "chunk": []byte(fmt.Sprintf("%d", i)), "chunks": []byte(fmt.Sprintf("%d", len(slices)))},
				})
			}
			i += 1
		}
	} else {
		objs = append(objs, v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:   key,
				Labels: lbs.toMap(),
			},
			Type: "helm.sh/release.v1",
			Data: map[string][]byte{"release": []byte(s)},
		})
	}
	return &objs, nil
}

// Load remaining chunks given a key and 1st release Secret
//func (secrets *Secrets) loadRemainingChunks(key string, obj *v1.Secret) ([]byte, error) {
//	chunks, _ := strconv.Atoi(string(obj.Data["chunks"]))
//	for chunk := 2 ; chunk <= chunks ; chunk++ {
//			key := fmt.Sprintf("%s.%d", obj.ObjectMeta.Name, chunk)
//			chunkobj, err := secrets.impl.Get(context.Background(), key, metav1.GetOptions{})
//			if err != nil {
//				if apierrors.IsNotFound(err) {
//					return nil, ErrReleaseNotFound
//				}
//				return nil, errors.Wrapf(err, "get: failed to get %q", key)
//			}
//			obj.Data["release"] = append(obj.Data["release"], chunkobj.Data["release"]...)
//	}
//	return obj.Data["release"], nil
//}
