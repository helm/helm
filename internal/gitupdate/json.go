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

package gitupdate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"

	jsonpatch "github.com/evanphx/json-patch/v5"
)

func applyJSON(original []byte, opts EditOptions) ([]byte, error) {
	document, err := decodeJSON(original)
	if err != nil {
		return nil, fmt.Errorf("decode target JSON: %w", err)
	}
	segments, err := parsePointer(opts.Pointer)
	if err != nil {
		return nil, err
	}

	var changed bool
	switch opts.Method {
	case MethodJSONSet:
		value, valueErr := decodeJSON(opts.Content)
		if valueErr != nil {
			return nil, fmt.Errorf("decode JSON content: %w", valueErr)
		}
		document, changed, err = setJSONValue(document, segments, value, opts.CreatePath)
	case MethodJSONMerge:
		value, valueErr := decodeJSON(opts.Content)
		if valueErr != nil {
			return nil, fmt.Errorf("decode JSON content: %w", valueErr)
		}
		document, changed, err = mergeJSONValue(document, segments, value, opts.CreatePath)
	case MethodJSONDelete:
		document, changed, err = deleteJSONValue(document, segments)
	default:
		return nil, fmt.Errorf("unsupported JSON method %q", opts.Method)
	}
	if err != nil {
		return nil, err
	}
	if !changed {
		return original, nil
	}

	updated, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode target JSON: %w", err)
	}
	return append(updated, '\n'), nil
}

func applyJSONPatch(original []byte, opts EditOptions) ([]byte, error) {
	originalValue, err := decodeJSON(original)
	if err != nil {
		return nil, fmt.Errorf("decode target JSON: %w", err)
	}

	var updated []byte
	switch opts.Method {
	case MethodJSONPatch:
		patch, err := jsonpatch.DecodePatch(opts.Content)
		if err != nil {
			return nil, fmt.Errorf("decode RFC 6902 JSON patch: %w", err)
		}
		updated, err = patch.Apply(original)
		if err != nil {
			return nil, fmt.Errorf("apply RFC 6902 JSON patch: %w", err)
		}
	case MethodJSONMergePatch:
		if _, err := decodeJSON(opts.Content); err != nil {
			return nil, fmt.Errorf("decode RFC 7386 JSON merge patch: %w", err)
		}
		updated, err = jsonpatch.MergePatch(original, opts.Content)
		if err != nil {
			return nil, fmt.Errorf("apply RFC 7386 JSON merge patch: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported JSON patch method %q", opts.Method)
	}

	updatedValue, err := decodeJSON(updated)
	if err != nil {
		return nil, fmt.Errorf("decode patched JSON: %w", err)
	}
	if reflect.DeepEqual(originalValue, updatedValue) {
		return original, nil
	}
	return preserveJSONNewline(original, updated), nil
}

func decodeJSON(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, err
	}
	return value, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("multiple JSON values are not allowed")
	}
	return err
}

func setJSONValue(current any, segments []string, value any, createPath bool) (any, bool, error) {
	if len(segments) == 0 {
		return value, !reflect.DeepEqual(current, value), nil
	}

	segment := segments[0]
	switch typed := current.(type) {
	case map[string]any:
		child, exists := typed[segment]
		if !exists {
			if len(segments) == 1 {
				typed[segment] = value
				return typed, true, nil
			}
			if !createPath {
				return current, false, fmt.Errorf("path component %q does not exist", segment)
			}
			child = map[string]any{}
		}
		updated, changed, err := setJSONValue(child, segments[1:], value, createPath)
		if err != nil {
			return current, false, err
		}
		if changed {
			typed[segment] = updated
		}
		return typed, changed, nil
	case []any:
		if segment == "-" {
			if len(segments) != 1 {
				return current, false, errors.New("array append token '-' is only valid at the final path component")
			}
			return append(typed, value), true, nil
		}
		index, err := parseArrayIndex(segment, len(typed))
		if err != nil {
			return current, false, err
		}
		updated, changed, err := setJSONValue(typed[index], segments[1:], value, createPath)
		if err != nil {
			return current, false, err
		}
		if changed {
			typed[index] = updated
		}
		return typed, changed, nil
	default:
		return current, false, fmt.Errorf("cannot traverse path component %q through %T", segment, current)
	}
}

func mergeJSONValue(current any, segments []string, value any, createPath bool) (any, bool, error) {
	if len(segments) != 0 {
		target, err := getJSONValue(current, segments)
		if err != nil {
			if !createPath {
				return current, false, err
			}
			valueMap, ok := value.(map[string]any)
			if !ok {
				return current, false, errors.New("json-merge content must be an object")
			}
			return setJSONValue(current, segments, valueMap, true)
		}
		merged, changed, err := deepMergeJSON(target, value)
		if err != nil || !changed {
			return current, changed, err
		}
		updated, _, err := setJSONValue(current, segments, merged, createPath)
		return updated, true, err
	}
	return deepMergeJSON(current, value)
}

func deepMergeJSON(target, source any) (any, bool, error) {
	targetMap, ok := target.(map[string]any)
	if !ok {
		return target, false, errors.New("json-merge target must be an object")
	}
	sourceMap, ok := source.(map[string]any)
	if !ok {
		return target, false, errors.New("json-merge content must be an object")
	}

	changed := false
	for key, sourceValue := range sourceMap {
		targetValue, exists := targetMap[key]
		if !exists {
			targetMap[key] = sourceValue
			changed = true
			continue
		}
		if _, sourceIsMap := sourceValue.(map[string]any); sourceIsMap {
			if _, targetIsMap := targetValue.(map[string]any); targetIsMap {
				merged, nestedChanged, err := deepMergeJSON(targetValue, sourceValue)
				if err != nil {
					return target, false, err
				}
				targetMap[key] = merged
				changed = changed || nestedChanged
				continue
			}
		}
		if !reflect.DeepEqual(targetValue, sourceValue) {
			targetMap[key] = sourceValue
			changed = true
		}
	}
	return targetMap, changed, nil
}

func getJSONValue(current any, segments []string) (any, error) {
	for _, segment := range segments {
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[segment]
			if !ok {
				return nil, fmt.Errorf("path component %q does not exist", segment)
			}
			current = next
		case []any:
			index, err := parseArrayIndex(segment, len(typed))
			if err != nil {
				return nil, err
			}
			current = typed[index]
		default:
			return nil, fmt.Errorf("cannot traverse path component %q through %T", segment, current)
		}
	}
	return current, nil
}

func deleteJSONValue(current any, segments []string) (any, bool, error) {
	if len(segments) == 0 {
		return current, false, errors.New("deleting the document root is not supported")
	}
	if len(segments) == 1 {
		switch typed := current.(type) {
		case map[string]any:
			if _, ok := typed[segments[0]]; !ok {
				return current, false, fmt.Errorf("path component %q does not exist", segments[0])
			}
			delete(typed, segments[0])
			return typed, true, nil
		case []any:
			index, err := parseArrayIndex(segments[0], len(typed))
			if err != nil {
				return current, false, err
			}
			return append(typed[:index], typed[index+1:]...), true, nil
		default:
			return current, false, fmt.Errorf("cannot delete path component %q from %T", segments[0], current)
		}
	}

	segment := segments[0]
	switch typed := current.(type) {
	case map[string]any:
		child, ok := typed[segment]
		if !ok {
			return current, false, fmt.Errorf("path component %q does not exist", segment)
		}
		updated, changed, err := deleteJSONValue(child, segments[1:])
		if changed {
			typed[segment] = updated
		}
		return typed, changed, err
	case []any:
		index, err := parseArrayIndex(segment, len(typed))
		if err != nil {
			return current, false, err
		}
		updated, changed, err := deleteJSONValue(typed[index], segments[1:])
		if changed {
			typed[index] = updated
		}
		return typed, changed, err
	default:
		return current, false, fmt.Errorf("cannot traverse path component %q through %T", segment, current)
	}
}

func parseArrayIndex(segment string, length int) (int, error) {
	index, err := strconv.Atoi(segment)
	if err != nil || index < 0 {
		return 0, fmt.Errorf("array path component %q is not a non-negative index", segment)
	}
	if index >= length {
		return 0, fmt.Errorf("array index %d is out of bounds (length %d)", index, length)
	}
	return index, nil
}

func preserveJSONNewline(original, updated []byte) []byte {
	if len(original) > 0 && original[len(original)-1] == '\n' && (len(updated) == 0 || updated[len(updated)-1] != '\n') {
		return append(updated, '\n')
	}
	return updated
}
