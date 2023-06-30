package driver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"helm.sh/helm/v3/pkg/release"
	"io"
	"net/http"
	"strconv"
	"testing"
)

type mockS3Client struct {
	getObjectCounter       int
	getObjectOverwrite     func(params *s3.GetObjectInput) (*s3.GetObjectOutput, error)
	putObjectCounter       int
	putObjectOverwrite     func(params *s3.PutObjectInput) (*s3.PutObjectOutput, error)
	headObjectCounter      int
	headObjectOverwite     func(params *s3.HeadObjectInput) (*s3.HeadObjectOutput, error)
	deleteObjectCounter    int
	deleteObjectOverwrite  func(params *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error)
	listObjectsV2Counter   int
	listObjectsV2Overwrite func(*s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error)
}

func (m *mockS3Client) GetObject(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	m.getObjectCounter++
	return m.getObjectOverwrite(params)
}
func (m *mockS3Client) PutObject(_ context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	m.putObjectCounter++
	return m.putObjectOverwrite(params)
}
func (m *mockS3Client) HeadObject(_ context.Context, params *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	m.headObjectCounter++
	return m.headObjectOverwite(params)
}
func (m *mockS3Client) DeleteObject(_ context.Context, params *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	m.deleteObjectCounter++
	return m.deleteObjectOverwrite(params)
}
func (m *mockS3Client) ListObjectsV2(_ context.Context, params *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	m.listObjectsV2Counter++
	return m.listObjectsV2Overwrite(params)
}

func TestCreate(t *testing.T) {
	ns := "test-namespace"
	bucket := "test-bucket"
	releaseName := "test-release"
	key := "test-release-1"
	expectedKey := fmt.Sprintf("%s/%s", ns, key)
	version := 1

	// Create a mock S3 client
	mockClient := &mockS3Client{
		putObjectOverwrite: func(params *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
			if *params.Bucket != bucket {
				t.Errorf("expected %s as bucket name got %s", bucket, *params.Bucket)
			}

			if *params.Key != expectedKey {
				t.Errorf("expected %s as key but got %s", expectedKey, *params.Key)
			}

			if params.Metadata["name"] != releaseName {
				t.Errorf("Expected metadata name key with value %s but got %s", releaseName, params.Metadata["Name"])
			}

			if params.Metadata["owner"] != "helm" {
				t.Errorf("Expected metadata owner key with value helm but got %s", params.Metadata["owner"])
			}

			if params.Metadata["version"] != strconv.Itoa(version) {
				t.Errorf("Expected metadata version key with value %d but got %s", version, params.Metadata["version"])
			}

			if params.Metadata["status"] != release.StatusPendingUpgrade.String() {
				t.Errorf("Expected metadata status key with value %s but got %s", release.StatusPendingUpgrade.String(), params.Metadata["status"])
			}

			return nil, nil
		},
		headObjectOverwite: func(params *s3.HeadObjectInput) (*s3.HeadObjectOutput, error) {
			if *params.Bucket != bucket {
				t.Errorf("expected %s as bucket name got %s", bucket, *params.Bucket)
			}

			if *params.Key != expectedKey {
				t.Errorf("expected %s as key but got %s", expectedKey, *params.Key)
			}

			respErr := smithyhttp.ResponseError{
				Response: &smithyhttp.Response{
					Response: &http.Response{
						StatusCode: 404,
					},
				},
			}

			return nil, &awshttp.ResponseError{
				ResponseError: &respErr,
			}
		},
	}

	// Create an instance of S3Driver with the mock client
	driver := &S3Driver{
		bucket:    bucket,
		namespace: ns,
		client:    mockClient,
		Log:       func(string, ...interface{}) {},
	}

	// Set up the input for the test
	rls := &release.Release{
		Name:    releaseName,
		Version: version,
		Info:    &release.Info{Status: release.StatusPendingUpgrade},
	}

	// Call the Create function
	err := driver.Create(key, rls)

	// Check if the Create function returned an error
	if err != nil {
		t.Fatalf("Create returned an unexpected error: %v", err)
	}

	if mockClient.putObjectCounter != 1 {
		t.Errorf("Expected PutObject to be called once but was called %d", mockClient.putObjectCounter)
	}

	if mockClient.headObjectCounter != 1 {
		t.Errorf("Expected HeadObject to be called once but was called %d", mockClient.headObjectCounter)
	}
}

func TestUpdate(t *testing.T) {
	namespace := "test-namespace"
	releaseName := "release"
	version := 1
	key := "test-key"
	bucket := "test-bucket"
	bucketKey := fmt.Sprintf("%s/%s", namespace, key)

	mockClient := &mockS3Client{
		putObjectOverwrite: func(params *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
			if *params.Key != bucketKey {
				t.Errorf("Expected key %s but got %s", bucketKey, *params.Key)
			}

			if *params.Bucket != bucket {
				t.Errorf("Expected bucket %s but got %s", bucket, *params.Bucket)
			}

			if params.Metadata["name"] != releaseName {
				t.Errorf("Expected metadata name key with value %s but got %s", releaseName, params.Metadata["Name"])
			}

			if params.Metadata["owner"] != "helm" {
				t.Errorf("Expected metadata owner key with value helm but got %s", params.Metadata["owner"])
			}

			if params.Metadata["version"] != strconv.Itoa(version) {
				t.Errorf("Expected metadata version key with value %d but got %s", version, params.Metadata["version"])
			}

			if params.Metadata["status"] != release.StatusDeployed.String() {
				t.Errorf("Expected metadata status key with value %s but got %s", release.StatusDeployed.String(), params.Metadata["status"])
			}

			return &s3.PutObjectOutput{}, nil
		},
	}

	// Create an instance of S3Driver with the mock client
	driver := &S3Driver{
		bucket:    bucket,
		namespace: namespace,
		client:    mockClient,
		Log:       func(string, ...interface{}) {},
	}

	// Set up the input for the test
	rls := &release.Release{
		Name:    releaseName,
		Version: version,
		Info:    &release.Info{Status: release.StatusDeployed},
	}

	// Call the Update function
	err := driver.Update(key, rls)

	// Check if the Update function returned an error
	if err != nil {
		t.Fatalf("Update returned an unexpected error: %v", err)
	}

	if mockClient.putObjectCounter != 1 {
		t.Errorf("Expected PutObject to be called once but was called %d", mockClient.putObjectCounter)
	}
}

func releaseToGetObjectOutput(rls *release.Release) *s3.GetObjectOutput {
	b, _ := encodeRelease(rls)
	bReader := bytes.NewReader([]byte(b))
	readCloser := io.NopCloser(bReader)

	return &s3.GetObjectOutput{
		Body: readCloser,
		Metadata: map[string]string{
			"name":    rls.Name,
			"owner":   "helm",
			"version": strconv.Itoa(rls.Version),
			"status":  release.StatusDeployed.String(),
		},
	}
}

func TestGet(t *testing.T) {
	key := "test-key"
	releaseName := "test-release"
	version := 1
	namespace := "test-namespace"
	bucket := "test-bucket"

	mockClient := &mockS3Client{
		getObjectOverwrite: func(params *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
			if *params.Bucket != bucket {
				t.Errorf("Expected bucket %s but got %s", bucket, *params.Bucket)
			}

			expectedKey := fmt.Sprintf("%s/%s", namespace, key)
			if *params.Key != expectedKey {
				t.Errorf("Expected key %s but got %s", expectedKey, *params.Key)
			}

			rls := &release.Release{
				Name:    releaseName,
				Version: version,
				Info:    &release.Info{Status: release.StatusDeployed},
			}

			return releaseToGetObjectOutput(rls), nil
		},
	}

	// Create an instance of S3Driver with the mock client
	driver := &S3Driver{
		bucket:    bucket,
		namespace: namespace,
		client:    mockClient,
		Log:       func(string, ...interface{}) {},
	}

	resp, err := driver.Get(key)
	if err != nil {
		t.Fatalf("Get returned an unexpected error %v", err)
	}

	if mockClient.getObjectCounter != 1 {
		t.Errorf("Expected GetObject to be called once but was called %d", mockClient.getObjectCounter)
	}

	if resp.Name != releaseName {
		t.Errorf("Expected release name %s but got %s", releaseName, resp.Name)
	}

	if resp.Version != version {
		t.Errorf("Expected version %d but got %d", version, resp.Version)
	}

	if resp.Info.Status != release.StatusDeployed {
		t.Errorf("Expected status %v but got %v", release.StatusDeployed, resp.Info.Status)
	}

	if resp.Labels["name"] != releaseName {
		t.Errorf("Expected metadata name key with value %s but got %s", releaseName, resp.Labels["Name"])
	}

	if resp.Labels["owner"] != "helm" {
		t.Errorf("Expected metadata owner key with value helm but got %s", resp.Labels["owner"])
	}

	if resp.Labels["version"] != strconv.Itoa(version) {
		t.Errorf("Expected metadata version key with value %d but got %s", version, resp.Labels["version"])
	}

	if resp.Labels["status"] != release.StatusDeployed.String() {
		t.Errorf("Expected metadata status key with value %s but got %s", release.StatusDeployed.String(), resp.Labels["status"])
	}
}

func mockReleases() map[string]release.Release {
	return map[string]release.Release{
		"test-namespace/first-release-1":       *releaseStub("first-release", 1, "test-namespace", release.StatusDeployed),
		"test-namespace/second-release-2":      *releaseStub("second-release", 2, "test-namespace", release.StatusPendingUpgrade),
		"other-test-namespace/third-release-3": *releaseStub("third-release", 3, "other-test-namespace", release.StatusFailed),
	}
}

func TestList(t *testing.T) {
	bucket := "test-bucket"
	rls := mockReleases()

	mockClient := &mockS3Client{
		listObjectsV2Overwrite: func(input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
			if input.Prefix != nil {
				t.Errorf("Expected empty prefix in ListObjectsV2 but got %s", *input.Prefix)
			}
			if *input.Bucket != bucket {
				t.Errorf("Expected bucket %s but got %s", bucket, *input.Bucket)
			}

			lstResult := []types.Object{}
			for s, _ := range rls {
				lstResult = append(lstResult, types.Object{
					Key: aws.String(s),
				})
			}

			return &s3.ListObjectsV2Output{
				IsTruncated: false,
				Contents:    lstResult,
			}, nil
		},
		getObjectOverwrite: func(params *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
			if value, exists := rls[*params.Key]; exists {
				return releaseToGetObjectOutput(&value), nil
			}

			t.Fatalf("GetObject was called with %s doesn't exist in releases", *params.Key)
			return nil, errors.New("GetObject was called with wrong key")
		},
	}

	driver := &S3Driver{
		bucket:    bucket,
		namespace: "",
		client:    mockClient,
		Log:       func(string, ...interface{}) {},
	}

	resp, err := driver.List(func(r *release.Release) bool {
		return true
	})
	if err != nil {
		t.Fatalf("List returned an unexpected error %v", err)
	}

	if len(resp) != 3 {
		t.Errorf("Expected 3 releases to be returned but got only %d", len(resp))
	}
}

func TestQuery(t *testing.T) {
	bucket := "test-bucket"
	rls := mockReleases()

	mockClient := &mockS3Client{
		listObjectsV2Overwrite: func(input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
			lstResult := []types.Object{}
			for s := range rls {
				lstResult = append(lstResult, types.Object{
					Key: aws.String(s),
				})
			}

			return &s3.ListObjectsV2Output{
				IsTruncated: false,
				Contents:    lstResult,
			}, nil
		},
		getObjectOverwrite: func(params *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
			if value, exists := rls[*params.Key]; exists {
				return releaseToGetObjectOutput(&value), nil
			}

			t.Fatalf("GetObject was called with %s doesn't exist in releases", *params.Key)
			return nil, errors.New("GetObject was called with wrong key")
		},
	}

	driver := &S3Driver{
		bucket:    bucket,
		namespace: "",
		client:    mockClient,
		Log:       func(string, ...interface{}) {},
	}

	resp, err := driver.Query(map[string]string{"name": "first-release"})
	if err != nil {
		t.Fatalf("Query returned an unexpected error %v", err)
	}

	if len(resp) != 1 {
		t.Fatalf("Expected 1 releases to be returned but got only %d", len(resp))
	}

	if resp[0].Name != "first-release" {
		t.Errorf("Expected the only release to be available to be named first-release but got %s", resp[0].Name)
	}
}
