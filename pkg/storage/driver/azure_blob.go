/*
Copyright The Helm Authorab.

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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
	rspb "helm.sh/helm/v3/pkg/release"
)

var _ Driver = (*AzureBlob)(nil)

// SQL is the sql storage driver implementation.
type AzureBlob struct {
	container azblob.ContainerURL
	namespace string
	Log       func(string, ...interface{})
}

// Name returns the name of the driver.
func (ab *AzureBlob) Name() string {
	return "azure_blob"
}

// Get a container url
//https://pkg.go.dev/github.com/Azure/azure-storage-blob-go/azblob#example-package
func NewAzureBlob(accountName, accountKey string, logger func(string, ...interface{}), namespace string) (*AzureBlob, error) {
	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		return nil, err
	}

	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	//why isn't there a helper to parse this from a connection string?
	u, err := url.Parse(fmt.Sprintf("https://%ab.blob.core.windowab.net", accountName))
	if err != nil {
		return nil, err
	}
	serviceURL := azblob.NewServiceURL(*u, p)

	//should we segregate namespaces into different containers?
	//Have to list containers and parallel query if we go down that route
	containerURL := serviceURL.NewContainerURL("helmreleases")

	// Create the container on the service (with no metadata and no public access)
	_, err = containerURL.Create(context.TODO(), azblob.Metadata{}, azblob.PublicAccessNone)
	azErr, ok := err.(azblob.StorageError)
	if !ok || azErr.ServiceCode() != azblob.ServiceCodeContainerAlreadyExists {
		return nil, err
	}

	d := &AzureBlob{
		container: containerURL,
		Log:       logger,
		namespace: namespace,
	}

	return d, nil
}

func keyAndNS(namespace, key string) string {
	return strings.Join([]string{namespace, key}, "/")
}

// Get returns the release named by key.
func (ab *AzureBlob) Get(key string) (*rspb.Release, error) {
	blobURL := ab.container.NewBlockBlobURL(keyAndNS(ab.namespace, key))
	return get(blobURL)
}

func get(blobURL azblob.BlockBlobURL) (*rspb.Release, error) {
	get, err := blobURL.Download(context.TODO(), 0, 0, azblob.BlobAccessConditions{}, false)
	if err != nil {
		return nil, err
	}

	reader := get.Body(azblob.RetryReaderOptions{})
	defer reader.Close() // The client must close the response body when finished with it
	var rls rspb.Release
	if err := json.NewDecoder(reader).Decode(&rls); err != nil {
		return nil, err
	}

	return &rls, nil
}

// List returns the list of all releases such that filter(release) == true
func (ab *AzureBlob) List(filter func(*rspb.Release) bool) ([]*rspb.Release, error) {
	//do we need to filter on owner? it's always the same
	//Where(sq.Eq{sqlReleaseTableOwnerColumn: sqlReleaseDefaultOwner})

	releases := []*rspb.Release{}
	// If a namespace was specified, we only list releases from that namespace
	opt := azblob.ListBlobsSegmentOptions{Prefix: ab.namespace}
	for marker := (azblob.Marker{}); marker.NotDone(); { // The parens around Marker{} are required to avoid compiler error.
		// Get a result segment starting with the blob indicated by the current Marker.
		listBlob, err := ab.container.ListBlobsFlatSegment(context.TODO(), marker, opt)
		if err != nil {
			return nil, err
		}
		// IMPORTANT: ListBlobs returns the start of the next segment; you MUST use this to get
		// the next segment (after processing the current result segment).
		marker = listBlob.NextMarker

		//this is obviously pretty slow we couild parallize and use a wait/errorgroup
		for _, blobInfo := range listBlob.Segment.BlobItems {
			r, err := get(ab.container.NewBlockBlobURL(blobInfo.Name))
			if err != nil {
				return nil, err
			}
			if filter(r) {
				releases = append(releases, r)
			}
		}
	}
	return releases, nil
}

// Query returns the set of releases that match the provided set of labels.
func (ab *AzureBlob) Query(labels map[string]string) ([]*rspb.Release, error) {
	opt := azblob.ListBlobsSegmentOptions{Prefix: ab.namespace}
	releases := []*rspb.Release{}
	for marker := (azblob.Marker{}); marker.NotDone(); { // The parens around Marker{} are required to avoid compiler error.
		// Get a result segment starting with the blob indicated by the current Marker.
		listBlob, err := ab.container.ListBlobsFlatSegment(context.TODO(), marker, opt)
		if err != nil {
			return nil, err
		}
		// IMPORTANT: ListBlobs returns the start of the next segment; you MUST use this to get
		// the next segment (after processing the current result segment).
		marker = listBlob.NextMarker

		//this is obviously pretty slow we couild parallize and use a wait/errorgroup
		for _, blobInfo := range listBlob.Segment.BlobItems {
			//this logic is questionable. Unittest.
			labelmatch := true
			for _, tag := range blobInfo.BlobTags.BlobTagSet {
				val, ok := labels[tag.Key]
				if ok && val != tag.Value {
					labelmatch = false
					break
				}
			}
			if !labelmatch {
				continue
			}
			r, err := get(ab.container.NewBlockBlobURL(blobInfo.Name))
			if err != nil {
				return nil, err
			}
			releases = append(releases, r)
		}
	}
	return releases, nil
}

// Create creates a new release.
func (ab *AzureBlob) Update(key string, rls *rspb.Release) error {

	namespace := rls.Namespace
	if namespace == "" {
		namespace = defaultNamespace //why not ab.namespace?
	}
	//ab.namespace = namespace
	//thought this was wierd
	//What is this namespace we store here will this change future gets?
	//Following sql but this seems wrong

	blobURL := ab.container.NewBlockBlobURL(keyAndNS(namespace, key))

	tags := azblob.BlobTagsMap{
		sqlReleaseTableNameColumn:       rls.Name,
		sqlReleaseTableVersionColumn:    strconv.Itoa(rls.Version),
		sqlReleaseTableStatusColumn:     rls.Info.Status.String(),
		sqlReleaseTableOwnerColumn:      sqlReleaseDefaultOwner,
		sqlReleaseTableModifiedAtColumn: strconv.FormatInt(time.Now().Unix(), 10),
	}

	//gziping this is debatable for blob. We might just burn cpu for little gain
	b, err := json.Marshal(&rls)
	if err != nil {
		ab.Log("failed to encode release: %v", err)
		return err
	}

	headers := azblob.BlobHTTPHeaders{ContentType: "application/javascript", ContentEncoding: "gzip"}

	_, err = blobURL.Upload(context.TODO(), bytes.NewReader(b),
		headers, azblob.Metadata{}, azblob.BlobAccessConditions{}, azblob.DefaultAccessTier, tags)
	return err
}

// Create updates a release.
func (ab *AzureBlob) Create(key string, rls *rspb.Release) error {
	//fail if this already exists?
	return ab.Update(key, rls)
}

// Delete deletes a release or returns ErrReleaseNotFound.
func (ab *AzureBlob) Delete(key string) (*rspb.Release, error) {
	blobURL := ab.container.NewBlockBlobURL(keyAndNS(ab.namespace, key))
	r, err := get(blobURL)
	if err != nil {
		return nil, err
	}
	_, err = blobURL.Delete(context.TODO(), azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{})
	return r, err
}
