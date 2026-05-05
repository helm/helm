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

package copystructure

import (
	"fmt"
	"reflect"
)

// Copy performs a deep copy of the given src.
// This implementation handles the specific use cases needed by Helm.
func Copy(src any) (any, error) {
	if src == nil {
		return make(map[string]any), nil
	}
	return copyValue(reflect.ValueOf(src))
}

// copyValue handles copying using reflection for non-map types
func copyValue(original reflect.Value) (any, error) {
	switch original.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
		reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128, reflect.String, reflect.Array:
		return original.Interface(), nil

	case reflect.Interface:
		if original.IsNil() {
			return original.Interface(), nil
		}
		return copyValue(original.Elem())

	case reflect.Map:
		if original.IsNil() {
			return original.Interface(), nil
		}
		copied := reflect.MakeMap(original.Type())

		var err error
		var child any
		iter := original.MapRange()
		for iter.Next() {
			key := iter.Key()
			value := iter.Value()

			if value.Kind() == reflect.Interface && value.IsNil() {
				copied.SetMapIndex(key, value)
				continue
			}

			child, err = copyValue(value)
			if err != nil {
				return nil, err
			}
			copied.SetMapIndex(key, reflect.ValueOf(child))
		}
		return copied.Interface(), nil

	case reflect.Pointer:
		if original.IsNil() {
			return original.Interface(), nil
		}
		copied, err := copyValue(original.Elem())
		if err != nil {
			return nil, err
		}
		ptr := reflect.New(original.Type().Elem())
		ptr.Elem().Set(reflect.ValueOf(copied))
		return ptr.Interface(), nil

	case reflect.Slice:
		if original.IsNil() {
			return original.Interface(), nil
		}
		copied := reflect.MakeSlice(original.Type(), original.Len(), original.Cap())
		for i := 0; i < original.Len(); i++ {
			elem := original.Index(i)

			// Handle nil values in slices (e.g., interface{} elements that are nil)
			if elem.Kind() == reflect.Interface && elem.IsNil() {
				copied.Index(i).Set(elem)
				continue
			}

			val, err := copyValue(elem)
			if err != nil {
				return nil, err
			}
			copied.Index(i).Set(reflect.ValueOf(val))
		}
		return copied.Interface(), nil

	case reflect.Struct:
		copied := reflect.New(original.Type()).Elem()
		for i := 0; i < original.NumField(); i++ {
			elem, err := copyValue(original.Field(i))
			if err != nil {
				return nil, err
			}
			copied.Field(i).Set(reflect.ValueOf(elem))
		}
		return copied.Interface(), nil

	case reflect.Func, reflect.Chan, reflect.UnsafePointer:
		if original.IsNil() {
			return original.Interface(), nil
		}
		return original.Interface(), nil

	default:
		return original.Interface(), fmt.Errorf("unsupported type %v", original)
	}
}
