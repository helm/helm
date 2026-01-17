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

package chart

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	v3chart "helm.sh/helm/v4/internal/chart/v3"
	common "helm.sh/helm/v4/pkg/chart/common"
	v2chart "helm.sh/helm/v4/pkg/chart/v2"
)

var NewAccessor func(chrt Charter) (Accessor, error) = NewDefaultAccessor //nolint:revive

func NewDefaultAccessor(chrt Charter) (Accessor, error) {
	switch v := chrt.(type) {
	case v2chart.Chart:
		return &v2Accessor{&v}, nil
	case *v2chart.Chart:
		return &v2Accessor{v}, nil
	case v3chart.Chart:
		return &v3Accessor{&v}, nil
	case *v3chart.Chart:
		return &v3Accessor{v}, nil
	default:
		return nil, errors.New("unsupported chart type")
	}
}

type v2Accessor struct {
	chrt *v2chart.Chart
}

func (r *v2Accessor) Name() string {
	return r.chrt.Metadata.Name
}

func (r *v2Accessor) IsRoot() bool {
	return r.chrt.IsRoot()
}

func (r *v2Accessor) MetadataAsMap() map[string]interface{} {
	var ret map[string]interface{}
	if r.chrt.Metadata == nil {
		return ret
	}

	ret, err := structToMap(r.chrt.Metadata)
	if err != nil {
		slog.Error("error converting metadata to map", "error", err)
	}
	return ret
}

func (r *v2Accessor) Files() []*common.File {
	return r.chrt.Files
}

func (r *v2Accessor) Templates() []*common.File {
	return r.chrt.Templates
}

func (r *v2Accessor) ChartFullPath() string {
	return r.chrt.ChartFullPath()
}

func (r *v2Accessor) IsLibraryChart() bool {
	return strings.EqualFold(r.chrt.Metadata.Type, "library")
}

func (r *v2Accessor) Dependencies() []Charter {
	var deps = make([]Charter, len(r.chrt.Dependencies()))
	for i, c := range r.chrt.Dependencies() {
		deps[i] = c
	}
	return deps
}

func (r *v2Accessor) MetaDependencies() []Dependency {
	var deps = make([]Dependency, len(r.chrt.Metadata.Dependencies))
	for i, c := range r.chrt.Metadata.Dependencies {
		deps[i] = c
	}
	return deps
}

func (r *v2Accessor) Values() map[string]interface{} {
	return r.chrt.Values
}

func (r *v2Accessor) Schema() []byte {
	return r.chrt.Schema
}

func (r *v2Accessor) Deprecated() bool {
	return r.chrt.Metadata.Deprecated
}

type v3Accessor struct {
	chrt *v3chart.Chart
}

func (r *v3Accessor) Name() string {
	return r.chrt.Metadata.Name
}

func (r *v3Accessor) IsRoot() bool {
	return r.chrt.IsRoot()
}

func (r *v3Accessor) MetadataAsMap() map[string]interface{} {
	var ret map[string]interface{}
	if r.chrt.Metadata == nil {
		return ret
	}

	ret, err := structToMap(r.chrt.Metadata)
	if err != nil {
		slog.Error("error converting metadata to map", "error", err)
	}
	return ret
}

func (r *v3Accessor) Files() []*common.File {
	return r.chrt.Files
}

func (r *v3Accessor) Templates() []*common.File {
	return r.chrt.Templates
}

func (r *v3Accessor) ChartFullPath() string {
	return r.chrt.ChartFullPath()
}

func (r *v3Accessor) IsLibraryChart() bool {
	return strings.EqualFold(r.chrt.Metadata.Type, "library")
}

func (r *v3Accessor) Dependencies() []Charter {
	var deps = make([]Charter, len(r.chrt.Dependencies()))
	for i, c := range r.chrt.Dependencies() {
		deps[i] = c
	}
	return deps
}

func (r *v3Accessor) MetaDependencies() []Dependency {
	var deps = make([]Dependency, len(r.chrt.Dependencies()))
	for i, c := range r.chrt.Metadata.Dependencies {
		deps[i] = c
	}
	return deps
}

func (r *v3Accessor) Values() map[string]interface{} {
	return r.chrt.Values
}

func (r *v3Accessor) Schema() []byte {
	return r.chrt.Schema
}

func (r *v3Accessor) Deprecated() bool {
	return r.chrt.Metadata.Deprecated
}

func structToMap(obj interface{}) (map[string]interface{}, error) {
	objValue := reflect.ValueOf(obj)

	// If the value is a pointer, dereference it
	if objValue.Kind() == reflect.Pointer {
		objValue = objValue.Elem()
	}

	// Check if the input is a struct
	if objValue.Kind() != reflect.Struct {
		return nil, fmt.Errorf("input must be a struct or a pointer to a struct")
	}

	result := make(map[string]interface{})
	objType := objValue.Type()

	for i := 0; i < objValue.NumField(); i++ {
		field := objType.Field(i)
		value := objValue.Field(i)

		switch value.Kind() {
		case reflect.Struct:
			nestedMap, err := structToMap(value.Interface())
			if err != nil {
				return nil, err
			}
			result[field.Name] = nestedMap
		case reflect.Pointer:
			// Recurse for pointers by dereferencing
			if value.IsNil() {
				result[field.Name] = nil
			} else {
				nestedMap, err := structToMap(value.Interface())
				if err != nil {
					return nil, err
				}
				result[field.Name] = nestedMap
			}
		case reflect.Slice:
			sliceOfMaps := make([]interface{}, value.Len())
			for j := 0; j < value.Len(); j++ {
				sliceElement := value.Index(j)
				if sliceElement.Kind() == reflect.Struct || sliceElement.Kind() == reflect.Pointer {
					nestedMap, err := structToMap(sliceElement.Interface())
					if err != nil {
						return nil, err
					}
					sliceOfMaps[j] = nestedMap
				} else {
					sliceOfMaps[j] = sliceElement.Interface()
				}
			}
			result[field.Name] = sliceOfMaps
		default:
			result[field.Name] = value.Interface()
		}
	}
	return result, nil
}
