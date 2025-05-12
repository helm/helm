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
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"

	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/pusher"
)

type MockedPusher struct {
	mock.Mock
}

func (m *MockedPusher) Push(chartRef string, url string, _ ...pusher.Option) error {
	m.Called(chartRef, url)
	return nil
}

type MockedProviders struct {
	mock.Mock
}

func (m *MockedProviders) ByScheme(string) (pusher.Pusher, error) {
	args := m.Called()
	mockedPusher := args.Get(0).(pusher.Pusher)
	return mockedPusher, nil
}

func TestChartUploader_UploadTo_Happy(t *testing.T) {
	mockedPusher := new(MockedPusher)
	mockedPusher.On("Push").Return(nil)

	mockedProviders := new(MockedProviders)
	mockedProviders.On("ByScheme").Return(mockedPusher, nil)

	uploader := ChartUploader{
		Pushers: mockedProviders,
	}

	mockedPusher.On("Push", "testdata/test-0.1.0.tgz", "oci://test").Return(nil)
	err := uploader.UploadTo("testdata/test-0.1.0.tgz", "oci://test")
	mockedPusher.AssertCalled(t, "Push", "testdata/test-0.1.0.tgz", "oci://test")

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
