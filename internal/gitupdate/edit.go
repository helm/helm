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
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Method identifies how content is applied to the target file.
type Method string

const (
	MethodOverwrite      Method = "overwrite"
	MethodReplace        Method = "replace"
	MethodInsertBefore   Method = "insert-before"
	MethodInsertAfter    Method = "insert-after"
	MethodAppend         Method = "append"
	MethodPrepend        Method = "prepend"
	MethodYAMLSet        Method = "yaml-set"
	MethodYAMLMerge      Method = "yaml-merge"
	MethodYAMLDelete     Method = "yaml-delete"
	MethodJSONSet        Method = "json-set"
	MethodJSONMerge      Method = "json-merge"
	MethodJSONDelete     Method = "json-delete"
	MethodJSONPatch      Method = "json-patch"
	MethodJSONMergePatch Method = "json-merge-patch"
)

// ValidMethods returns the supported file editing methods.
func ValidMethods() []string {
	return []string{
		string(MethodOverwrite),
		string(MethodReplace),
		string(MethodInsertBefore),
		string(MethodInsertAfter),
		string(MethodAppend),
		string(MethodPrepend),
		string(MethodYAMLSet),
		string(MethodYAMLMerge),
		string(MethodYAMLDelete),
		string(MethodJSONSet),
		string(MethodJSONMerge),
		string(MethodJSONDelete),
		string(MethodJSONPatch),
		string(MethodJSONMergePatch),
	}
}

// EditOptions describes one deterministic edit to a file.
type EditOptions struct {
	Method          Method
	Content         []byte
	ContentProvided bool
	LiteralMatch    string
	RegexMatch      string
	ExpectedMatches int
	AllMatches      bool
	Expand          bool
	Pointer         string
	CreatePath      bool
}

// ApplyFile applies an edit to targetPath underneath worktreeRoot. It rejects
// paths that escape the worktree, paths inside .git, and symbolic links.
func ApplyFile(worktreeRoot, targetPath string, opts EditOptions, createFile bool) (bool, error) {
	targetPath, err := validateTargetPath(targetPath)
	if err != nil {
		return false, err
	}

	root, err := os.OpenRoot(worktreeRoot)
	if err != nil {
		return false, fmt.Errorf("open worktree: %w", err)
	}
	defer root.Close()

	localPath := filepath.FromSlash(targetPath)
	if err := rejectSymlinks(root, localPath); err != nil {
		return false, err
	}

	existed := true
	original, err := root.ReadFile(localPath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return false, fmt.Errorf("read target file %q: %w", targetPath, err)
		}
		if !createFile {
			return false, fmt.Errorf("target file %q does not exist (use --create-file to create it)", targetPath)
		}
		existed = false
		original = initialContent(opts.Method)
	}

	updated, err := Apply(original, opts)
	if err != nil {
		return false, fmt.Errorf("edit %q: %w", targetPath, err)
	}
	if existed && bytes.Equal(original, updated) {
		return false, nil
	}

	parent := filepath.Dir(localPath)
	if parent != "." {
		if err := root.MkdirAll(parent, 0o755); err != nil {
			return false, fmt.Errorf("create parent directories for %q: %w", targetPath, err)
		}
	}

	mode := fs.FileMode(0o644)
	if info, statErr := root.Stat(localPath); statErr == nil {
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf("target path %q is not a regular file", targetPath)
		}
		mode = info.Mode().Perm()
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return false, fmt.Errorf("inspect target file %q: %w", targetPath, statErr)
	}

	if err := root.WriteFile(localPath, updated, mode); err != nil {
		return false, fmt.Errorf("write target file %q: %w", targetPath, err)
	}
	return true, nil
}

func validateTargetPath(targetPath string) (string, error) {
	if targetPath == "" {
		return "", errors.New("target file is required")
	}
	if filepath.IsAbs(targetPath) || path.IsAbs(filepath.ToSlash(targetPath)) {
		return "", fmt.Errorf("target file %q must be relative to the repository root", targetPath)
	}

	clean := path.Clean(filepath.ToSlash(targetPath))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("target file %q escapes the repository root", targetPath)
	}
	for segment := range strings.SplitSeq(clean, "/") {
		if strings.EqualFold(segment, ".git") {
			return "", fmt.Errorf("target file %q must not be inside .git", targetPath)
		}
	}
	return clean, nil
}

func rejectSymlinks(root *os.Root, targetPath string) error {
	current := ""
	for segment := range strings.SplitSeq(filepath.Clean(targetPath), string(filepath.Separator)) {
		if current == "" {
			current = segment
		} else {
			current = filepath.Join(current, segment)
		}

		info, err := root.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect target path %q: %w", filepath.ToSlash(current), err)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("target path %q contains symbolic link %q", filepath.ToSlash(targetPath), filepath.ToSlash(current))
		}
	}
	return nil
}

func initialContent(method Method) []byte {
	switch method {
	case MethodYAMLSet, MethodYAMLMerge, MethodYAMLDelete:
		return []byte("{}\n")
	case MethodJSONSet, MethodJSONMerge, MethodJSONDelete, MethodJSONPatch, MethodJSONMergePatch:
		return []byte("{}\n")
	default:
		return nil
	}
}

// Apply returns the edited file contents without writing to disk.
func Apply(original []byte, opts EditOptions) ([]byte, error) {
	if err := validateEditOptions(opts); err != nil {
		return nil, err
	}

	switch opts.Method {
	case MethodOverwrite:
		return append([]byte(nil), opts.Content...), nil
	case MethodAppend:
		return append(append([]byte(nil), original...), opts.Content...), nil
	case MethodPrepend:
		return append(append([]byte(nil), opts.Content...), original...), nil
	case MethodReplace, MethodInsertBefore, MethodInsertAfter:
		return applySelectedText(original, opts)
	case MethodYAMLSet, MethodYAMLMerge, MethodYAMLDelete:
		return applyYAML(original, opts)
	case MethodJSONSet, MethodJSONMerge, MethodJSONDelete:
		return applyJSON(original, opts)
	case MethodJSONPatch, MethodJSONMergePatch:
		return applyJSONPatch(original, opts)
	default:
		return nil, fmt.Errorf("unknown edit method %q", opts.Method)
	}
}

func validateEditOptions(opts EditOptions) error {
	if !slices.Contains(ValidMethods(), string(opts.Method)) {
		return fmt.Errorf("unknown edit method %q (supported: %s)", opts.Method, strings.Join(ValidMethods(), ", "))
	}
	if opts.ExpectedMatches < 0 {
		return errors.New("expected matches must be zero or greater")
	}

	needsContent := opts.Method != MethodYAMLDelete && opts.Method != MethodJSONDelete
	if needsContent && !opts.ContentProvided {
		return fmt.Errorf("method %q requires content", opts.Method)
	}
	if !needsContent && opts.ContentProvided {
		return fmt.Errorf("method %q does not accept content", opts.Method)
	}

	isSelectedText := opts.Method == MethodReplace || opts.Method == MethodInsertBefore || opts.Method == MethodInsertAfter
	hasLiteral := opts.LiteralMatch != ""
	hasRegex := opts.RegexMatch != ""
	if isSelectedText {
		if hasLiteral == hasRegex {
			return fmt.Errorf("method %q requires exactly one of a non-empty literal match or regular expression", opts.Method)
		}
	} else if hasLiteral || hasRegex {
		return fmt.Errorf("method %q does not accept a text selector", opts.Method)
	}
	if opts.AllMatches && !isSelectedText {
		return fmt.Errorf("method %q does not accept --all", opts.Method)
	}

	isStructuredPath := opts.Method == MethodYAMLSet || opts.Method == MethodYAMLMerge || opts.Method == MethodYAMLDelete ||
		opts.Method == MethodJSONSet || opts.Method == MethodJSONMerge || opts.Method == MethodJSONDelete
	if isStructuredPath {
		if _, err := parsePointer(opts.Pointer); err != nil {
			return err
		}
	} else if opts.Pointer != "" {
		return fmt.Errorf("method %q does not accept a JSON pointer", opts.Method)
	}
	canCreatePath := opts.Method == MethodYAMLSet || opts.Method == MethodYAMLMerge ||
		opts.Method == MethodJSONSet || opts.Method == MethodJSONMerge
	if opts.CreatePath && !canCreatePath {
		return fmt.Errorf("method %q does not accept --create-path", opts.Method)
	}

	if opts.Expand && opts.RegexMatch == "" {
		return errors.New("capture expansion requires a regular expression selector")
	}
	return nil
}

func applySelectedText(original []byte, opts EditOptions) ([]byte, error) {
	if opts.LiteralMatch != "" {
		return applyLiteralSelector(original, opts)
	}
	return applyRegexSelector(original, opts)
}

func applyLiteralSelector(original []byte, opts EditOptions) ([]byte, error) {
	match := []byte(opts.LiteralMatch)
	actual := bytes.Count(original, match)
	if err := validateMatchCount(actual, opts.ExpectedMatches); err != nil {
		return nil, err
	}

	replacement := opts.Content
	switch opts.Method {
	case MethodInsertBefore:
		replacement = append(append([]byte(nil), opts.Content...), match...)
	case MethodInsertAfter:
		replacement = append(append([]byte(nil), match...), opts.Content...)
	case MethodReplace:
	default:
		return nil, fmt.Errorf("unsupported literal text method %q", opts.Method)
	}

	limit := 1
	if opts.AllMatches {
		limit = -1
	}
	return bytes.Replace(original, match, replacement, limit), nil
}

func applyRegexSelector(original []byte, opts EditOptions) ([]byte, error) {
	expression, err := regexp.Compile(opts.RegexMatch)
	if err != nil {
		return nil, fmt.Errorf("compile regular expression: %w", err)
	}

	matches := expression.FindAllSubmatchIndex(original, -1)
	if err := validateMatchCount(len(matches), opts.ExpectedMatches); err != nil {
		return nil, err
	}
	if !opts.AllMatches {
		matches = matches[:1]
	}

	var result bytes.Buffer
	last := 0
	for _, indexes := range matches {
		start, end := indexes[0], indexes[1]
		result.Write(original[last:start])
		if opts.Method == MethodInsertAfter {
			result.Write(original[start:end])
		}
		if opts.Expand {
			result.Write(expression.Expand(nil, opts.Content, original, indexes))
		} else {
			result.Write(opts.Content)
		}
		if opts.Method == MethodInsertBefore {
			result.Write(original[start:end])
		}
		last = end
	}
	result.Write(original[last:])
	return result.Bytes(), nil
}

func validateMatchCount(actual, expected int) error {
	if actual == 0 {
		return errors.New("selector did not match the target file")
	}
	if expected > 0 && actual != expected {
		return fmt.Errorf("selector matched %d times; expected %d", actual, expected)
	}
	return nil
}
