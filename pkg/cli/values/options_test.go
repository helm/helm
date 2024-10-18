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
	"reflect"
	"testing"

	"helm.sh/helm/v3/pkg/getter"
)

func Test_mergeMaps(t *testing.T) {
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
	var p getter.Providers
	filePath := "%a.txt"
	_, err := readFile(filePath, p)
	if err == nil {
		t.Errorf("Expected error when has special strings")
	}
}

func TestOptions_MergeValues(t *testing.T) {
	const (
		crewNameKey      = `crew`
		shipNameKey      = `ship`
		powerUserskey    = `power-users`
		nonPowerUsersKey = `non-power-users`
		strawHatsCrew    = `Straw Hat Pirates`
		strawHatsShip1   = `Going Merry`
		strawHatsShip2   = `Thousand Sunny`
	)

	var (
		powerUsersVal = []interface{}{
			"Luffy",
			"Chopper",
			"Robin",
			"Brook",
		}
		nonPowerUsersVal = []interface{}{
			"Zoro",
			"Nami",
			"Ussop",
			"Sanji",
			"Franky",
			"Jinbei",
		}
	)

	type args struct {
		p getter.Providers
	}
	tests := []struct {
		name    string
		opts    Options
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "--values-directory with single level",
			opts: Options{
				ValueFiles: []string{},
				ValuesDirectories: []string{
					"testdata/chart-with-values-dir/values.d",
				},
				StringValues: []string{},
				Values:       []string{},
				FileValues:   []string{},
				JSONValues:   []string{},
			},
			args: args{
				p: []getter.Provider{},
			},
			want: map[string]interface{}{
				powerUserskey:    powerUsersVal,
				nonPowerUsersKey: nonPowerUsersVal,
			},
			wantErr: false,
		},
		{
			name: "--values-directory with nested directories",
			opts: Options{
				ValueFiles: []string{},
				ValuesDirectories: []string{
					"testdata/multi-level-values-dir/values.d",
				},
				StringValues: []string{},
				Values:       []string{},
				FileValues:   []string{},
				JSONValues:   []string{},
			},
			args: args{
				p: []getter.Provider{},
			},
			want: map[string]interface{}{
				crewNameKey:      strawHatsCrew,
				shipNameKey:      strawHatsShip1,
				powerUserskey:    powerUsersVal,
				nonPowerUsersKey: nonPowerUsersVal,
			},
			wantErr: false,
		},
		{
			name: "--values-directory value overwritten by --values",
			opts: Options{
				ValueFiles: []string{
					"testdata/multi-level-values-dir/ship.yaml",
				},
				ValuesDirectories: []string{
					"testdata/multi-level-values-dir/values.d",
				},
				StringValues: []string{},
				Values:       []string{},
				FileValues:   []string{},
				JSONValues:   []string{},
			},
			args: args{
				p: []getter.Provider{},
			},
			want: map[string]interface{}{
				crewNameKey:      strawHatsCrew,
				shipNameKey:      strawHatsShip2, // This is the value overwritten by values file "ship.yaml"
				powerUserskey:    powerUsersVal,
				nonPowerUsersKey: nonPowerUsersVal,
			},
			wantErr: false,
		},
		{
			name: "--values-directory with missing directory",
			opts: Options{
				ValueFiles: []string{},
				ValuesDirectories: []string{
					"testdata/chart-with-values-dir/non-existing/",
				},
				StringValues: []string{},
				Values:       []string{},
				FileValues:   []string{},
				JSONValues:   []string{},
			},
			args: args{
				p: []getter.Provider{},
			},
			want:    map[string]interface{}(nil),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.opts.MergeValues(tt.args.p)

			if (err != nil) != tt.wantErr {
				t.Errorf("Options.MergeValues() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Expected result from MergeValues() = %v, got %v", tt.want, got)
			}
		})
	}
}
