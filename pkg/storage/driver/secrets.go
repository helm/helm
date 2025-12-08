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

package driver // import "helm.sh/helm/v4/pkg/storage/driver"

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kblabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"helm.sh/helm/v4/internal/logging"
	"helm.sh/helm/v4/pkg/release"
	rspb "helm.sh/helm/v4/pkg/release/v1"
)

var _ Driver = (*Secrets)(nil)

// SecretsDriverName is the string name of the driver.
const SecretsDriverName = "Secret"

// Secrets is a wrapper around an implementation of a kubernetes
// SecretsInterface.
type Secrets struct {
	impl corev1.SecretInterface
	// Embed a LogHolder to provide logger functionality
	logging.LogHolder
}

// NewSecrets initializes a new Secrets wrapping an implementation of
// the kubernetes SecretsInterface.
func NewSecrets(impl corev1.SecretInterface) *Secrets {
	s := &Secrets{
		impl: impl,
	}
	s.SetLogger(slog.Default().Handler())
	return s
}

// Name returns the name of the driver.
func (secrets *Secrets) Name() string {
	return SecretsDriverName
}

// Get fetches the release named by key. The corresponding release is returned
// or error if not found.
func (secrets *Secrets) Get(key string) (release.Releaser, error) {
	// fetch the secret holding the release named by key
	obj, err := secrets.impl.Get(context.Background(), key, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, ErrReleaseNotFound
		}
		return nil, fmt.Errorf("get: failed to get %q: %w", key, err)
	}
	// found the secret, decode the base64 data string
	r, err := decodeRelease(string(obj.Data["release"]))
	if err != nil {
		return r, fmt.Errorf("get: failed to decode data %q: %w", key, err)
	}
	r.Labels = filterSystemLabels(obj.Labels)
	return r, nil
}

// List fetches all releases and returns the list releases such
// that filter(release) == true. An error is returned if the
// secret fails to retrieve the releases.
func (secrets *Secrets) List(filter func(release.Releaser) bool) ([]release.Releaser, error) {
	lsel := kblabels.Set{"owner": "helm"}.AsSelector()
	opts := metav1.ListOptions{LabelSelector: lsel.String()}

	list, err := secrets.impl.List(context.Background(), opts)
	if err != nil {
		return nil, fmt.Errorf("list: failed to list: %w", err)
	}

	var results []release.Releaser

	// iterate over the secrets object list
	// and decode each release
	for _, item := range list.Items {
		rls, err := decodeRelease(string(item.Data["release"]))
		if err != nil {
			secrets.Logger().Debug(
				"list failed to decode release", slog.String("key", item.Name),
				slog.Any("error", err),
			)
			continue
		}

		rls.Labels = item.Labels

		if filter(rls) {
			results = append(results, rls)
		}
	}
	return results, nil
}

// Query fetches all releases that match the provided map of labels.
// An error is returned if the secret fails to retrieve the releases.
func (secrets *Secrets) Query(labels map[string]string) ([]release.Releaser, error) {
	ls := kblabels.Set{}
	for k, v := range labels {
		if errs := validation.IsValidLabelValue(v); len(errs) != 0 {
			return nil, fmt.Errorf("invalid label value: %q: %s", v, strings.Join(errs, "; "))
		}
		ls[k] = v
	}

	opts := metav1.ListOptions{LabelSelector: ls.AsSelector().String()}

	list, err := secrets.impl.List(context.Background(), opts)
	if err != nil {
		return nil, fmt.Errorf("query: failed to query with labels: %w", err)
	}

	if len(list.Items) == 0 {
		return nil, ErrReleaseNotFound
	}

	var results []release.Releaser
	for _, item := range list.Items {
		rls, err := decodeRelease(string(item.Data["release"]))
		if err != nil {
			secrets.Logger().Debug(
				"failed to decode release",
				slog.String("key", item.Name),
				slog.Any("error", err),
			)
			continue
		}
		rls.Labels = item.Labels
		results = append(results, rls)
	}
	return results, nil
}

// Create creates a new Secret holding the release. If the
// Secret already exists, ErrReleaseExists is returned.
func (secrets *Secrets) Create(key string, rel release.Releaser) error {
	// set labels for secrets object meta data
	var lbs labels

	rls, err := releaserToV1Release(rel)
	if err != nil {
		return err
	}

	lbs.init()
	lbs.fromMap(rls.Labels)
	lbs.set("createdAt", fmt.Sprintf("%v", time.Now().Unix()))

	// create a new secret to hold the release
	obj, err := newSecretsObject(key, rls, lbs)
	if err != nil {
		return fmt.Errorf("create: failed to encode release %q: %w", rls.Name, err)
	}
	// push the secret object out into the kubiverse
	if _, err := secrets.impl.Create(context.Background(), obj, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return ErrReleaseExists
		}

		return fmt.Errorf("create: failed to create: %w", err)
	}
	return nil
}

// Update updates the Secret holding the release. If not found
// the Secret is created to hold the release.
func (secrets *Secrets) Update(key string, rel release.Releaser) error {
	// set labels for secrets object meta data
	var lbs labels

	rls, err := releaserToV1Release(rel)
	if err != nil {
		return err
	}

	lbs.init()
	lbs.fromMap(rls.Labels)
	lbs.set("modifiedAt", fmt.Sprintf("%v", time.Now().Unix()))

	// create a new secret object to hold the release
	obj, err := newSecretsObject(key, rls, lbs)
	if err != nil {
		return fmt.Errorf("update: failed to encode release %q: %w", rls.Name, err)
	}
	// push the secret object out into the kubiverse
	_, err = secrets.impl.Update(context.Background(), obj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update: failed to update: %w", err)
	}
	return nil
}

// Delete deletes the Secret holding the release named by key.
func (secrets *Secrets) Delete(key string) (rls release.Releaser, err error) {
	// fetch the release to check existence
	if rls, err = secrets.Get(key); err != nil {
		return nil, err
	}
	// delete the release
	err = secrets.impl.Delete(context.Background(), key, metav1.DeleteOptions{})
	if err != nil {
		return nil, err
	}
	return rls, nil
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
func newSecretsObject(key string, rls *rspb.Release, lbs labels) (*v1.Secret, error) {
	const owner = "helm"

	// encode the release
	s, err := encodeRelease(rls)
	if err != nil {
		return nil, err
	}

	if lbs == nil {
		lbs.init()
	}

	// apply custom labels
	lbs.fromMap(rls.Labels)

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
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   key,
			Labels: lbs.toMap(),
		},
		Type: "helm.sh/release.v1",
		Data: map[string][]byte{"release": []byte(s)},
	}, nil
}
