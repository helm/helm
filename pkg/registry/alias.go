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

package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"
)

// Aliases represents the registry/aliases.yaml file
type Aliases struct {
	APIVersion    string            `json:"apiVersion"`
	Aliases       map[string]string `json:"aliases"`
	Substitutions map[string]string `json:"substitutions"`
}

// NewAliasesFile generates an empty aliases file.
//
// APIVersion is automatically set.
func NewAliasesFile() *Aliases {
	return &Aliases{
		APIVersion:    APIVersionV1,
		Aliases:       map[string]string{},
		Substitutions: map[string]string{},
	}
}

// LoadAliasesFile takes a file at the given path and returns an Aliases object
func LoadAliasesFile(path string) (*Aliases, error) {
	a := NewAliasesFile()
	b, err := os.ReadFile(path)
	if err != nil {
		return a, fmt.Errorf("couldn't load aliases file (%s): %w", path, err)
	}

	err = yaml.Unmarshal(b, a)
	return a, err
}

// SetAlias adds or updates an alias.
func (a *Aliases) SetAlias(alias, url string) {
	a.Aliases[alias] = url
}

// RemoveAlias removes the entry from the list of repository aliases.
// RemoveAlias returns true if the alias existed before it was deleted.
func (a *Aliases) RemoveAlias(alias string) bool {
	_, existing := a.Aliases[alias]
	delete(a.Aliases, alias)

	return existing
}

// SetSubstitution adds or updates a substitution.
func (a *Aliases) SetSubstitution(substitution, replacement string) {
	a.Substitutions[substitution] = replacement
}

// RemoveSubstitution removes the substitution and returns true if the
// substitution existed before it was deleted.
func (a *Aliases) RemoveSubstitution(substitution string) bool {
	_, existing := a.Substitutions[substitution]
	delete(a.Substitutions, substitution)

	return existing
}

// Expand first expands aliases to their mapped value end then performs
// prefix substitutions until no substitution matches or each substitution
// was used at most once.
func (a *Aliases) Expand(source string) string {
	return a.performSubstitutions(a.expandAlias(source))
}

func (a *Aliases) expandAlias(source string) string {
	isAtAlias := strings.HasPrefix(source, "@")
	isLongAlias := strings.HasPrefix(source, "alias:")
	if isAtAlias || isLongAlias {
		var alias string
		if isAtAlias {
			alias = strings.TrimPrefix(source, "@")
		} else if isLongAlias {
			alias = strings.TrimPrefix(source, "alias:")
		}
		if v, existing := a.Aliases[alias]; existing {
			return v
		}
	}

	return source
}

func (a *Aliases) performSubstitutions(source string) string {
	current := source

	// no recursions
	used := make(map[string]bool, len(a.Substitutions))
	orderedSubstitutions := make([]string, 0, len(a.Substitutions))
	for k := range a.Substitutions {
		orderedSubstitutions = append(orderedSubstitutions, k)
	}
	sort.SliceStable(orderedSubstitutions, func(i, j int) bool {
		return len(orderedSubstitutions[i]) < len(orderedSubstitutions[j])
	})
	var changed bool
	for {
		changed = false
		for i := range orderedSubstitutions {
			k := orderedSubstitutions[i]
			if !used[k] && strings.HasPrefix(current, k) {
				used[k] = true
				current = a.Substitutions[k] + strings.TrimPrefix(current, k)
				changed = true
			}
		}
		if !changed {
			break
		}
	}

	return current
}

// WriteAliasesFile writes an aliases file to the given path.
func (a *Aliases) WriteAliasesFile(path string, perm os.FileMode) error {
	data, err := yaml.Marshal(a)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}
