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

package driver

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"

	rspb "helm.sh/helm/v3/pkg/release"
)

const (
	// GCSDriverName is the string name of this driver.
	GCSDriverName = "GCS"

	gcsReleaseNameMetadata       = "name"
	gcsReleaseNamespaceMetadata  = "namespace"
	gcsReleaseVersionMetadata    = "version"
	gcsReleaseStatusMetadata     = "status"
	gcsReleaseOwnerColumn        = "owner"
	gcsReleaseCreatedAtMetadata  = "createdAt"
	gcsReleaseModifiedAtMetadata = "modifiedAt"
)

// GCS is the GCS storage driver implementation.
type GCS struct {
	client *storage.Client

	bucket     string
	pathPrefix string

	namespace string

	now string

	Log func(string, ...interface{})
}

// NewGCS initializes a new GCS driver.
func NewGCS(bucket, pathPrefix, namespace string, logger func(string, ...interface{})) (*GCS, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	driver := &GCS{
		client: client,

		bucket:     bucket,
		pathPrefix: pathPrefix,
		namespace:  namespace,

		now: time.Now().Format("2006-01-02 15:04:05"),

		Log: logger,
	}

	return driver, nil
}

// Name returns the name of the driver.
func (s *GCS) Name() string {
	return GCSDriverName
}

// Get returns the release named by key or returns ErrReleaseNotFound.
func (s *GCS) Get(key string) (*rspb.Release, error) {
	rel, _, err := s.readRelease(s.fullPathName(key, s.namespace), false)
	return rel, err
}

// List returns the list of all releases such that filter(release) == true
func (s *GCS) List(filter func(*rspb.Release) bool) ([]*rspb.Release, error) {
	namespaces, err := s.listNamespaces()
	if err != nil {
		s.Log("list: failed to list: %v", err)
		return nil, err
	}

	var list []*rspb.Release

	for _, namespace := range namespaces {
		releases, err := s.listReleases(namespace)
		if err != nil {
			s.Log("list: failed to list releases in namespace: %v", namespace, err)
			continue
		}

		for _, key := range releases {
			release, _, err := s.readRelease(key, false)
			if err != nil {
				s.Log("list: failed to read release: %v: %v", release, err)
				continue
			}
			if filter(release) {
				list = append(list, release)
			}
		}
	}

	return list, nil
}

// Query returns the set of releases that match the provided set of labels
func (s *GCS) Query(labels map[string]string) ([]*rspb.Release, error) {
	// List namespaces
	namespaces, err := s.listNamespaces()
	if err != nil {
		s.Log("list: failed to list: %v", err)
		return nil, err
	}

	// Create filter to compare labels and metadata
	filter := func(metadata map[string]string) bool {
		for key, val := range labels {
			if metadataVal, ok := metadata[key]; !ok || metadataVal != val {
				return false
			}
		}

		return true
	}

	// List the releases
	var list []*rspb.Release
	for _, namespace := range namespaces {
		releases, err := s.listReleases(namespace)
		if err != nil {
			s.Log("list: failed to list releases in namespace: %v", namespace, err)
			continue
		}

		for _, key := range releases {
			release, metadata, err := s.readRelease(key, true)
			if err != nil {
				s.Log("list: failed to read release: %v: %v", release, err)
				continue
			}
			if filter(metadata) {
				list = append(list, release)
			}
		}
	}

	if len(list) == 0 {
		return nil, ErrReleaseNotFound
	}

	return list, nil
}

// Create creates a new release.
func (s *GCS) Create(key string, rls *rspb.Release) error {
	release, err := encodeRelease(rls)
	if err != nil {
		s.Log("failed to encode release: %v", err)
		return err
	}

	ctx := context.Background()
	obj := s.client.Bucket(s.bucket).
		Object(s.fullPathName(key, rls.Namespace)).
		If(storage.Conditions{DoesNotExist: true}).
		NewWriter(ctx)
	obj.Metadata = s.metadata(rls, true)

	if _, err := obj.Write([]byte(release)); err != nil {
		s.Log("failed to write object: %v", err)
		return err
	}

	if err := obj.Close(); err != nil {
		switch e := err.(type) {
		case *googleapi.Error:
			if e.Code == http.StatusPreconditionFailed {
				s.Log("release %s already exists", key)
				return ErrReleaseExists
			}
		default:
			s.Log("failed to close bucket: %v", err)
			return err
		}
	}

	return nil
}

// Update updates a release.
func (s *GCS) Update(key string, rls *rspb.Release) error {
	release, err := encodeRelease(rls)
	if err != nil {
		s.Log("failed to encode release: %v", err)
		return err
	}

	ctx := context.Background()
	obj := s.client.Bucket(s.bucket).Object(s.fullPathName(key, rls.Namespace)).NewWriter(ctx)
	obj.Metadata = s.metadata(rls, false)
	if _, err := obj.Write([]byte(release)); err != nil {
		s.Log("failed to write object: %v", err)
		return err
	}

	if err := obj.Close(); err != nil {
		s.Log("failed to close object: %v", err)
		return err
	}

	return nil
}

// Delete deletes a release or returns ErrReleaseNotFound.
func (s *GCS) Delete(key string) (*rspb.Release, error) {
	ctx := context.Background()
	obj := s.client.Bucket(s.bucket).Object(s.fullPathName(key, s.namespace))
	objRdr, err := obj.NewReader(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			err = ErrReleaseNotFound
		}
		return nil, err
	}

	record, err := ioutil.ReadAll(objRdr)
	objRdr.Close()
	if err != nil {
		return nil, err
	}

	release, err := decodeRelease(string(record))
	if err != nil {
		s.Log("get: failed to decode data %q: %v", key, err)
		return nil, err
	}

	if obj.Delete(ctx); err != nil {
		s.Log("failed to delete object: %v", err)
		return nil, err
	}

	return release, nil
}

// fullPathName returns the full path name composed by prefix, name and namespace
func (s *GCS) fullPathName(name, namespace string) string {
	if namespace == "" {
		namespace = defaultNamespace
	}
	return strings.TrimLeft(
		fmt.Sprintf("%s/%s/%s", s.pathPrefix, namespace, name),
		"/",
	)
}

// metadata returns the metadata list from release
func (s *GCS) metadata(rls *rspb.Release, isCreation bool) map[string]string {
	md := map[string]string{
		gcsReleaseNameMetadata:       rls.Name,
		gcsReleaseNamespaceMetadata:  rls.Namespace,
		gcsReleaseStatusMetadata:     rls.Info.Status.String(),
		gcsReleaseVersionMetadata:    strconv.Itoa(rls.Version),
		gcsReleaseOwnerColumn:        "helm",
		gcsReleaseModifiedAtMetadata: s.now,
	}
	if isCreation {
		md[gcsReleaseCreatedAtMetadata] = s.now
	}
	return md
}

// readRelease helps to read a release by the object key
func (s *GCS) readRelease(key string, withMetadata bool) (*rspb.Release, map[string]string, error) {
	metadata := make(map[string]string)
	ctx := context.Background()
	objHandle := s.client.Bucket(s.bucket).Object(key)
	obj, err := objHandle.NewReader(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			err = ErrReleaseNotFound
		}
		return nil, metadata, err
	}

	record, err := ioutil.ReadAll(obj)
	obj.Close()
	if err != nil {
		return nil, metadata, err
	}

	release, err := decodeRelease(string(record))
	if err != nil {
		s.Log("read release: failed to decode data %q: %v", key, err)
		return nil, metadata, err
	}

	if withMetadata {
		objAttrs, err := objHandle.Attrs(ctx)
		if err != nil {
			s.Log("read release: failed to read metadata %q: %v", key, err)
			return nil, metadata, err
		}
		metadata = objAttrs.Metadata
	}

	return release, metadata, nil
}

// listNamespaces helps to list namespaces inside a bucket
func (s *GCS) listNamespaces() ([]string, error) {
	if s.namespace != "" {
		return []string{s.namespace}, nil
	}

	objectsName := make(map[string]string)

	prefix := fmt.Sprintf("%s/", strings.TrimRight(s.pathPrefix, "/"))
	query := storage.Query{
		StartOffset: prefix,
		Prefix:      prefix,
	}
	ctx := context.Background()
	objs := s.client.Bucket(s.bucket).Objects(ctx, &query)
	for {
		objAttrs, err := objs.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			s.Log("unable to list objects", err)
			return []string{}, err
		}
		objectNameWithoutPrefix := strings.TrimPrefix(objAttrs.Name, prefix)
		eq := strings.IndexByte(objectNameWithoutPrefix, '/')
		objectName := objectNameWithoutPrefix[:eq]
		objectsName[objectName] = objectName
	}

	namespaces := []string{}
	for objectName := range objectsName {
		namespaces = append(namespaces, objectName)
	}

	return namespaces, nil
}

// listReleases helps to list releases in a namespace "folder"
func (s *GCS) listReleases(namespace string) ([]string, error) {
	releases := []string{}

	query := storage.Query{
		Prefix: strings.TrimRight(fmt.Sprintf("%s/%s", s.pathPrefix, namespace), "/"),
	}
	ctx := context.Background()
	objs := s.client.Bucket(s.bucket).Objects(ctx, &query)
	for {
		objAttrs, err := objs.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			s.Log("unable to list objects", err)
			return []string{}, err
		}

		releases = append(releases, objAttrs.Name)
	}

	return releases, nil
}
