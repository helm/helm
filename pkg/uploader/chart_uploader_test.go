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

package uploader

import (
	"crypto/sha256"
	"fmt"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/pusher"
	"helm.sh/helm/v4/pkg/registry"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestChartUploader_UploadTo_Happy(t *testing.T) {
	shasum := ""
	var content []byte
	contentSize := 0
	uploadSessionId := "c6ce3ba4-788f-4e10-93ed-ff77d35c6851"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.WriteHeader(http.StatusNotFound)
		} else if r.Method == "POST" && r.URL.Path == "/v2/test/blobs/uploads/" {
			w.Header().Set("Location", "/v2/test/blobs/uploads/"+uploadSessionId)
			w.WriteHeader(http.StatusAccepted)
		} else if r.Method == "PUT" && r.URL.Path == "/v2/test/blobs/uploads/"+uploadSessionId {
			w.Header().Set("Location", "/v2/test/blobs/sha256:irrelevant")
			w.WriteHeader(http.StatusCreated)
		} else if r.Method == "PUT" && strings.HasPrefix(r.URL.Path, "/v2/test/manifests/sha256:") {
			content = make([]byte, r.ContentLength)
			r.Body.Read(content)
			h := sha256.New()
			h.Write(content)
			shasumBuilder := strings.Builder{}
			fmt.Fprintf(&shasumBuilder, "%x", h.Sum(nil))
			shasum = shasumBuilder.String()
			contentSize = len(content)
			w.Header().Set("Location", "/v2/test/manifests/sha256:"+shasum)
			w.WriteHeader(http.StatusCreated)
		} else if r.Method == "GET" && r.URL.Path == "/v2/test/manifests/sha256:"+shasum {
			w.Header().Set("Content-Length", strconv.Itoa(contentSize))
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			w.Header().Set("Docker-Content-Digest", "sha256:"+shasum)
			_, err := fmt.Fprint(w, string(content))
			if err != nil {
				t.Errorf("%s", err)
			}
		} else if r.Method == "PUT" && r.URL.Path == "/v2/test/manifests/0.1.0" {
			w.Header().Set("Docker-Content-Digest", "sha256:"+shasum)
			w.Header().Set("Content-Length", strconv.Itoa(contentSize))
			w.Header().Set("Location", "/v2/test/manifests/sha256:"+shasum)
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer srv.Close()

	envSettings := cli.EnvSettings{}
	pushers := pusher.All(&envSettings)

	u, _ := url.ParseRequestURI(srv.URL)
	ociReplacedUrl := strings.Replace(u.String(), "http", "oci", 1)

	srvClient := srv.Client()
	client, _ := registry.NewClient(registry.ClientOptHTTPClient(srvClient))
	uploader := ChartUploader{
		Pushers:        pushers,
		RegistryClient: client,
		Options:        []pusher.Option{pusher.WithPlainHTTP(true)},
	}

	err := uploader.UploadTo("testdata/test-0.1.0.tgz", ociReplacedUrl+"")
	if err != nil {
		fmt.Println(err)
		t.Errorf("Expected push to succeed but got error")
	}
}

func TestChartUploader_UploadTo_InvalidChartUrlFormat(t *testing.T) {
	envSettings := cli.EnvSettings{}

	pushers := pusher.All(&envSettings)

	uploader := ChartUploader{
		Pushers: pushers,
	}

	err := uploader.UploadTo("main", "://invalid.com")
	const expectedError = "invalid chart URL format"
	fmt.Println(err.Error())
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '" + expectedError + "'")
	}
}

func TestChartUploader_UploadTo_SchemePrefixMissingFromRemote(t *testing.T) {
	envSettings := cli.EnvSettings{}

	pushers := pusher.All(&envSettings)

	uploader := ChartUploader{
		Pushers: pushers,
	}

	err := uploader.UploadTo("main", "invalid.com")
	const expectedError = "scheme prefix missing from remote"

	fmt.Println(err.Error())
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '" + expectedError + "'")
	}
}

func TestChartUploader_UploadTo_SchemeNotRegistered(t *testing.T) {
	envSettings := cli.EnvSettings{}

	pushers := pusher.All(&envSettings)

	uploader := ChartUploader{
		Pushers: pushers,
	}

	err := uploader.UploadTo("main", "grpc://invalid.com")
	const expectedError = "scheme \"grpc\" not supported"

	fmt.Println(err.Error())
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '" + expectedError + "'")
	}
}
