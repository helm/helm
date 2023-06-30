package driver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"helm.sh/helm/v3/pkg/release"
	"io"
	"net/http"
	"os"
	"strconv"
)

var _ Driver = (*S3Driver)(nil)

// S3DriverName is the string name of the driver.
const S3DriverName = "S3Driver"

type S3Client interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

// S3Driver is an implementation of the driver interface
type S3Driver struct {
	bucket    string
	namespace string
	client    S3Client
	Log       func(string, ...interface{})
}

// NewS3 initializes a new S3Driver an implementation of the driver interface
func NewS3(bucket string, logger func(string, ...interface{}), namespace string) (*S3Driver, error) {
	endpointResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		lUrl, lUrlExists := os.LookupEnv("HELM_DRIVER_S3_BUCKET_LOCATION_URL")
		if service == s3.ServiceID && lUrlExists {
			return aws.Endpoint{
				PartitionID:       "aws",
				URL:               lUrl,
				SigningRegion:     "us-east-2",
				HostnameImmutable: true,
			}, nil
		}
		// performs a fallback to the default endpoint resolver
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithEndpointResolverWithOptions(endpointResolver))
	if err != nil {
		logger("failed to load aws sdk configuration with error %v", err)
		return nil, err
	}

	client := s3.NewFromConfig(cfg, func(options *s3.Options) {
		pStyle, pStyleExists := os.LookupEnv("HELM_DRIVER_S3_USE_PATH_STYLE")
		if pStyleExists {
			val, err := strconv.ParseBool(pStyle)
			if err == nil {
				options.UsePathStyle = val
			}
		}
	})

	return &S3Driver{
		client:    client,
		bucket:    bucket,
		namespace: namespace,
		Log:       logger,
	}, nil

}

// Create creates a new release and stores it on S3. If the object already exists, ErrReleaseExists is returned.
func (s *S3Driver) Create(key string, rls *release.Release) error {
	s3Key := fmt.Sprintf("%s/%s", s.namespace, key)
	exists, err := s.pathAlreadyExists(s3Key)
	if err != nil {
		s.Log("Failed to check if release already exists with error %v", err)
		return err
	}

	if exists {
		return ErrReleaseExists
	}

	return s.Update(key, rls)
}

// pathAlreadyExists returns a boolean if the release at a specific path already exists
func (s *S3Driver) pathAlreadyExists(path string) (bool, error) {
	input := &s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    &path,
	}
	_, err := s.client.HeadObject(context.Background(), input)
	if err != nil {
		var responseError *awshttp.ResponseError
		if errors.As(err, &responseError) && responseError.ResponseError.HTTPStatusCode() == http.StatusNotFound {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// Update updates a release by using the Create function
func (s *S3Driver) Update(key string, rls *release.Release) error {
	s3Key := fmt.Sprintf("%s/%s", s.namespace, key)

	r, err := encodeRelease(rls)
	if err != nil {
		s.Log("Failed to decode release %q with error %v", key, err)
		return err
	}

	bodyBytes := []byte(r)
	input := &s3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    &s3Key,
		Body:   bytes.NewReader(bodyBytes),
		Metadata: map[string]string{
			"name":    rls.Name,
			"owner":   "helm",
			"status":  rls.Info.Status.String(),
			"version": strconv.Itoa(rls.Version),
		},
	}

	_, pErr := s.client.PutObject(context.Background(), input)

	return pErr
}

// Delete loads a release and deletes it from S3
func (s *S3Driver) Delete(key string) (*release.Release, error) {
	rel, err := s.Get(key)
	if err != nil {
		s.Log("failed to get %s with error %v", key, err)
		return nil, err
	}

	s3Key := fmt.Sprintf("%s/%s", s.namespace, key)
	input := &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    &s3Key,
	}

	_, delErr := s.client.DeleteObject(context.Background(), input)
	if err != nil {
		s.Log("failed to delete object %s with error %v", s3Key, err)
		return nil, delErr
	}

	return rel, nil
}

// Get loads a release based on the namespace & release name
func (s *S3Driver) Get(key string) (*release.Release, error) {
	s3Key := fmt.Sprintf("%s/%s", s.namespace, key)
	return s.getByPath(s3Key)
}

// getByPath internal method to load a release by path consisting of the pattern namespace/release.name
func (s *S3Driver) getByPath(path string) (*release.Release, error) {
	input := &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &path,
	}

	resp, err := s.client.GetObject(context.Background(), input)
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, ErrReleaseNotFound
		}

		s.Log("failed to get release with s3 key %s with error %v", path, err)
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	body := string(bodyBytes)
	rel, err := decodeRelease(body)
	if err != nil {
		s.Log("failed to decode release from s3 key %s with error %v", path, err)
		return nil, err
	}

	rel.Labels = resp.Metadata
	return rel, nil
}

// List lists all releases within the S3 Bucket limited by the s.namespace parameter to limit it to a specific namespace.
func (s *S3Driver) List(filter func(*release.Release) bool) ([]*release.Release, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: &s.bucket,
	}

	if len(s.namespace) != 0 {
		prefix := fmt.Sprintf("%s/", s.namespace)
		input.Prefix = &prefix
	}

	var releases []*release.Release
	paginator := s3.NewListObjectsV2Paginator(s.client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			s.Log("failed to paginate through s3 bucket %s with error %v", s.bucket, err)
			return nil, err
		}

		for _, obj := range page.Contents {
			key := obj.Key
			r, err := s.getByPath(*key)
			if err != nil {
				s.Log("list: failed to decode load release with key: %s: %v", key, err)
				continue
			}

			if filter(r) {
				releases = append(releases, r)
			}
		}
	}

	return releases, nil
}

// Query is an internal facade to the List function that limits the result to all objects that have the matching labels
func (s *S3Driver) Query(labels map[string]string) ([]*release.Release, error) {
	releases, err := s.List(func(r *release.Release) bool {
		for key, val := range labels {
			if r.Labels[key] != val {
				return false
			}
		}

		return true
	})

	if err != nil {
		s.Log("failed to query releases with error %v", err)
		return nil, err
	}

	if len(releases) == 0 {
		return nil, ErrReleaseNotFound
	}

	return releases, nil
}

// Name returns the name of the driver.
func (s *S3Driver) Name() string {
	return S3DriverName
}
