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

package values

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
)

func TestMergeValues(t *testing.T) {
	nestedMap := map[string]interface{}{
		"foo": "bar",
		"baz": map[string]string{
			"cool": "stuff",
		},
	}
	anotherNestedMap := map[string]interface{}{
		"foo": "bar",
		"baz": map[string]string{
			"cool":    "things",
			"awesome": "stuff",
		},
	}
	flatMap := map[string]interface{}{
		"foo": "bar",
		"baz": "stuff",
	}
	anotherFlatMap := map[string]interface{}{
		"testing": "fun",
	}

	testMap := mergeMaps(flatMap, nestedMap)
	equal := reflect.DeepEqual(testMap, nestedMap)
	if !equal {
		t.Errorf("Expected a nested map to overwrite a flat value. Expected: %v, got %v", nestedMap, testMap)
	}

	testMap = mergeMaps(nestedMap, flatMap)
	equal = reflect.DeepEqual(testMap, flatMap)
	if !equal {
		t.Errorf("Expected a flat value to overwrite a map. Expected: %v, got %v", flatMap, testMap)
	}

	testMap = mergeMaps(nestedMap, anotherNestedMap)
	equal = reflect.DeepEqual(testMap, anotherNestedMap)
	if !equal {
		t.Errorf("Expected a nested map to overwrite another nested map. Expected: %v, got %v", anotherNestedMap, testMap)
	}

	testMap = mergeMaps(anotherFlatMap, anotherNestedMap)
	expectedMap := map[string]interface{}{
		"testing": "fun",
		"foo":     "bar",
		"baz": map[string]string{
			"cool":    "things",
			"awesome": "stuff",
		},
	}
	equal = reflect.DeepEqual(testMap, expectedMap)
	if !equal {
		t.Errorf("Expected a map with different keys to merge properly with another map. Expected: %v, got %v", expectedMap, testMap)
	}
}

func TestReadFile(t *testing.T) {
	var settings = cli.New()
	p := getter.All(settings)
	testData := []byte("OK")

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(testData)
	}))
	defer s.Close()

	tmpFile, err := ioutil.TempFile("", "illegal-character-file-name-%s--*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmpFile.Write(testData); err != nil {
		t.Fatal(err)
	}
	defer tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	tests := []struct {
		name     string
		filePath string
		p        getter.Providers
		want     []byte
		wantErr  bool
	}{
		{
			name:     "local-file",
			filePath: tmpFile.Name(),
			p:        p,
			want:     testData,
			wantErr:  false,
		},
		{
			name:     "http-getter",
			filePath: s.URL,
			p:        p,
			want:     testData,
			wantErr:  false,
		},
		{
			name:     "not-supported-schema",
			filePath: "not-supported-schema://file",
			p:        p,
			want:     nil,
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readFile(tt.filePath, p)
			if (err != nil) != tt.wantErr {
				t.Errorf("readFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readFile() got = %v, want %v", got, tt.want)
			}
		})
	}
}
