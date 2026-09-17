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
	"errors"
	"fmt"
	"io"
	"reflect"

	"go.yaml.in/yaml/v3"
)

func applyYAML(original []byte, opts EditOptions) ([]byte, error) {
	document, err := decodeYAMLDocument(original)
	if err != nil {
		return nil, fmt.Errorf("decode target YAML: %w", err)
	}
	segments, err := parsePointer(opts.Pointer)
	if err != nil {
		return nil, err
	}

	root := document.Content[0]
	var changed bool
	switch opts.Method {
	case MethodYAMLSet:
		value, valueErr := decodeYAMLValue(opts.Content)
		if valueErr != nil {
			return nil, fmt.Errorf("decode YAML content: %w", valueErr)
		}
		changed, err = setYAMLValue(root, segments, value, opts.CreatePath)
	case MethodYAMLMerge:
		value, valueErr := decodeYAMLValue(opts.Content)
		if valueErr != nil {
			return nil, fmt.Errorf("decode YAML content: %w", valueErr)
		}
		changed, err = mergeYAMLAt(root, segments, value, opts.CreatePath)
	case MethodYAMLDelete:
		changed, err = deleteYAMLValue(root, segments)
	default:
		return nil, fmt.Errorf("unsupported YAML method %q", opts.Method)
	}
	if err != nil {
		return nil, err
	}
	if !changed {
		return original, nil
	}

	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(document); err != nil {
		return nil, fmt.Errorf("encode target YAML: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("finish encoding target YAML: %w", err)
	}
	return output.Bytes(), nil
}

func decodeYAMLDocument(data []byte) (*yaml.Node, error) {
	document, err := decodeOneYAML(data)
	if err != nil {
		return nil, err
	}
	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return nil, errors.New("expected one YAML document with a root value")
	}
	return document, nil
}

func decodeYAMLValue(data []byte) (*yaml.Node, error) {
	document, err := decodeOneYAML(data)
	if err != nil {
		return nil, err
	}
	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return nil, errors.New("expected one YAML value")
	}
	return cloneYAMLNode(document.Content[0]), nil
}

func decodeOneYAML(data []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	if len(document.Content) == 0 {
		return nil, errors.New("empty YAML is not a value")
	}

	var extra yaml.Node
	err := decoder.Decode(&extra)
	if err == nil {
		return nil, errors.New("multiple YAML documents are not supported")
	}
	if !errors.Is(err, io.EOF) {
		return nil, err
	}
	if err := validateYAMLUniqueKeys(document.Content[0]); err != nil {
		return nil, err
	}
	return &document, nil
}

func validateYAMLUniqueKeys(node *yaml.Node) error {
	if node.Kind == yaml.MappingNode {
		seen := map[string]struct{}{}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			encoded, err := yaml.Marshal(key)
			if err != nil {
				return fmt.Errorf("encode YAML mapping key: %w", err)
			}
			identity := string(encoded)
			if _, exists := seen[identity]; exists {
				return fmt.Errorf("duplicate YAML mapping key %q", key.Value)
			}
			seen[identity] = struct{}{}
		}
	}
	for _, child := range node.Content {
		if err := validateYAMLUniqueKeys(child); err != nil {
			return err
		}
	}
	return nil
}

func setYAMLValue(current *yaml.Node, segments []string, value *yaml.Node, createPath bool) (bool, error) {
	if len(segments) == 0 {
		if equalYAMLValue(current, value) {
			return false, nil
		}
		replacement := cloneYAMLNode(value)
		preserveYAMLComments(replacement, current)
		*current = *replacement
		return true, nil
	}

	segment := segments[0]
	switch current.Kind {
	case yaml.MappingNode:
		index := yamlMapValueIndex(current, segment)
		if index < 0 {
			if len(segments) == 1 {
				current.Content = append(current.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: segment},
					cloneYAMLNode(value),
				)
				return true, nil
			}
			if !createPath {
				return false, fmt.Errorf("path component %q does not exist", segment)
			}
			child := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			current.Content = append(current.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: segment},
				child,
			)
			return setYAMLValue(child, segments[1:], value, createPath)
		}
		return setYAMLValue(current.Content[index], segments[1:], value, createPath)
	case yaml.SequenceNode:
		if segment == "-" {
			if len(segments) != 1 {
				return false, errors.New("array append token '-' is only valid at the final path component")
			}
			current.Content = append(current.Content, cloneYAMLNode(value))
			return true, nil
		}
		index, err := parseArrayIndex(segment, len(current.Content))
		if err != nil {
			return false, err
		}
		return setYAMLValue(current.Content[index], segments[1:], value, createPath)
	case yaml.AliasNode:
		return false, fmt.Errorf("cannot traverse path component %q through a YAML alias", segment)
	default:
		return false, fmt.Errorf("cannot traverse path component %q through YAML kind %d", segment, current.Kind)
	}
}

func mergeYAMLAt(current *yaml.Node, segments []string, value *yaml.Node, createPath bool) (bool, error) {
	if value.Kind != yaml.MappingNode {
		return false, errors.New("yaml-merge content must be a mapping")
	}
	if len(segments) == 0 {
		return deepMergeYAML(current, value)
	}

	target, err := getYAMLValue(current, segments)
	if err != nil {
		if !createPath {
			return false, err
		}
		return setYAMLValue(current, segments, value, true)
	}
	return deepMergeYAML(target, value)
}

func deepMergeYAML(target, source *yaml.Node) (bool, error) {
	if target.Kind != yaml.MappingNode {
		return false, errors.New("yaml-merge target must be a mapping")
	}
	if source.Kind != yaml.MappingNode {
		return false, errors.New("yaml-merge content must be a mapping")
	}

	changed := false
	for i := 0; i < len(source.Content); i += 2 {
		sourceKey := source.Content[i]
		sourceValue := source.Content[i+1]
		if sourceKey.Kind != yaml.ScalarNode {
			return false, errors.New("yaml-merge only supports scalar mapping keys")
		}

		targetIndex := yamlMapValueIndex(target, sourceKey.Value)
		if targetIndex < 0 {
			target.Content = append(target.Content, cloneYAMLNode(sourceKey), cloneYAMLNode(sourceValue))
			changed = true
			continue
		}

		targetValue := target.Content[targetIndex]
		if targetValue.Kind == yaml.MappingNode && sourceValue.Kind == yaml.MappingNode {
			nestedChanged, err := deepMergeYAML(targetValue, sourceValue)
			if err != nil {
				return false, err
			}
			changed = changed || nestedChanged
			continue
		}
		if equalYAMLValue(targetValue, sourceValue) {
			continue
		}
		replacement := cloneYAMLNode(sourceValue)
		preserveYAMLComments(replacement, targetValue)
		target.Content[targetIndex] = replacement
		changed = true
	}
	return changed, nil
}

func getYAMLValue(current *yaml.Node, segments []string) (*yaml.Node, error) {
	for _, segment := range segments {
		switch current.Kind {
		case yaml.MappingNode:
			index := yamlMapValueIndex(current, segment)
			if index < 0 {
				return nil, fmt.Errorf("path component %q does not exist", segment)
			}
			current = current.Content[index]
		case yaml.SequenceNode:
			index, err := parseArrayIndex(segment, len(current.Content))
			if err != nil {
				return nil, err
			}
			current = current.Content[index]
		case yaml.AliasNode:
			return nil, fmt.Errorf("cannot traverse path component %q through a YAML alias", segment)
		default:
			return nil, fmt.Errorf("cannot traverse path component %q through YAML kind %d", segment, current.Kind)
		}
	}
	return current, nil
}

func deleteYAMLValue(current *yaml.Node, segments []string) (bool, error) {
	if len(segments) == 0 {
		return false, errors.New("deleting the document root is not supported")
	}
	if len(segments) == 1 {
		segment := segments[0]
		switch current.Kind {
		case yaml.MappingNode:
			valueIndex := yamlMapValueIndex(current, segment)
			if valueIndex < 0 {
				return false, fmt.Errorf("path component %q does not exist", segment)
			}
			keyIndex := valueIndex - 1
			current.Content = append(current.Content[:keyIndex], current.Content[valueIndex+1:]...)
			return true, nil
		case yaml.SequenceNode:
			index, err := parseArrayIndex(segment, len(current.Content))
			if err != nil {
				return false, err
			}
			current.Content = append(current.Content[:index], current.Content[index+1:]...)
			return true, nil
		default:
			return false, fmt.Errorf("cannot delete path component %q from YAML kind %d", segment, current.Kind)
		}
	}

	segment := segments[0]
	switch current.Kind {
	case yaml.MappingNode:
		index := yamlMapValueIndex(current, segment)
		if index < 0 {
			return false, fmt.Errorf("path component %q does not exist", segment)
		}
		return deleteYAMLValue(current.Content[index], segments[1:])
	case yaml.SequenceNode:
		index, err := parseArrayIndex(segment, len(current.Content))
		if err != nil {
			return false, err
		}
		return deleteYAMLValue(current.Content[index], segments[1:])
	case yaml.AliasNode:
		return false, fmt.Errorf("cannot traverse path component %q through a YAML alias", segment)
	default:
		return false, fmt.Errorf("cannot traverse path component %q through YAML kind %d", segment, current.Kind)
	}
}

func yamlMapValueIndex(mapping *yaml.Node, key string) int {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		keyNode := mapping.Content[i]
		if keyNode.Kind == yaml.ScalarNode && keyNode.Value == key {
			return i + 1
		}
	}
	return -1
}

func equalYAMLValue(left, right *yaml.Node) bool {
	var leftValue any
	if err := left.Decode(&leftValue); err != nil {
		return false
	}
	var rightValue any
	if err := right.Decode(&rightValue); err != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

func preserveYAMLComments(target, source *yaml.Node) {
	if target.HeadComment == "" {
		target.HeadComment = source.HeadComment
	}
	if target.LineComment == "" {
		target.LineComment = source.LineComment
	}
	if target.FootComment == "" {
		target.FootComment = source.FootComment
	}
}

func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	clones := map[*yaml.Node]*yaml.Node{}
	var clone func(*yaml.Node) *yaml.Node
	clone = func(source *yaml.Node) *yaml.Node {
		if source == nil {
			return nil
		}
		if existing, ok := clones[source]; ok {
			return existing
		}
		target := *source
		target.Content = nil
		target.Alias = nil
		clones[source] = &target
		for _, child := range source.Content {
			target.Content = append(target.Content, clone(child))
		}
		target.Alias = clone(source.Alias)
		return &target
	}
	return clone(node)
}
