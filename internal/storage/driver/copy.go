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

package driver

import (
	"bytes"
	"maps"
	"slices"

	"k8s.io/apimachinery/pkg/runtime"

	chart "helm.sh/helm/v4/internal/chart/v3"
	rspb "helm.sh/helm/v4/internal/release/v2"
	"helm.sh/helm/v4/pkg/chart/common"
)

// copyRelease returns a deep copy of rls.
//
// The persistent drivers serialize releases on write and deserialize them on
// read, so callers can never reach the stored data. The memory driver keeps
// releases as Go values, and therefore has to copy them explicitly to offer the
// same isolation.
//
// Values held in the release that Helm does not model, such as custom types
// stored in Config, are copied as interface values rather than recursively.
func copyRelease(rls *rspb.Release) *rspb.Release {
	if rls == nil {
		return nil
	}
	return &rspb.Release{
		Name:        rls.Name,
		Info:        copyInfo(rls.Info),
		Chart:       copyChart(rls.Chart),
		Config:      copyValues(rls.Config),
		Manifest:    rls.Manifest,
		Hooks:       copyHooks(rls.Hooks),
		Version:     rls.Version,
		Namespace:   rls.Namespace,
		Labels:      maps.Clone(rls.Labels),
		ApplyMethod: rls.ApplyMethod,
	}
}

func copyInfo(info *rspb.Info) *rspb.Info {
	if info == nil {
		return nil
	}
	out := *info
	if info.Resources != nil {
		out.Resources = make(map[string][]runtime.Object, len(info.Resources))
		for name, objs := range info.Resources {
			copied := make([]runtime.Object, len(objs))
			for i, obj := range objs {
				if obj != nil {
					copied[i] = obj.DeepCopyObject()
				}
			}
			out.Resources[name] = copied
		}
	}
	return &out
}

func copyHooks(hooks []*rspb.Hook) []*rspb.Hook {
	if hooks == nil {
		return nil
	}
	out := make([]*rspb.Hook, len(hooks))
	for i, hook := range hooks {
		if hook == nil {
			continue
		}
		copied := *hook
		copied.Events = slices.Clone(hook.Events)
		copied.DeletePolicies = slices.Clone(hook.DeletePolicies)
		copied.OutputLogPolicies = slices.Clone(hook.OutputLogPolicies)
		out[i] = &copied
	}
	return out
}

func copyChart(ch *chart.Chart) *chart.Chart {
	if ch == nil {
		return nil
	}
	out := &chart.Chart{
		Raw:           copyFiles(ch.Raw),
		Metadata:      copyMetadata(ch.Metadata),
		Lock:          copyLock(ch.Lock),
		Templates:     copyFiles(ch.Templates),
		Values:        copyValues(ch.Values),
		Schema:        bytes.Clone(ch.Schema),
		SchemaModTime: ch.SchemaModTime,
		Files:         copyFiles(ch.Files),
		ModTime:       ch.ModTime,
	}
	// AddDependency sets the parent of each dependency, which rebuilds the
	// chart tree without reaching into the unexported fields.
	for _, dep := range ch.Dependencies() {
		out.AddDependency(copyChart(dep))
	}
	return out
}

func copyMetadata(md *chart.Metadata) *chart.Metadata {
	if md == nil {
		return nil
	}
	out := *md
	out.Sources = slices.Clone(md.Sources)
	out.Keywords = slices.Clone(md.Keywords)
	out.Annotations = maps.Clone(md.Annotations)
	if md.Maintainers != nil {
		out.Maintainers = make([]*chart.Maintainer, len(md.Maintainers))
		for i, m := range md.Maintainers {
			if m == nil {
				continue
			}
			copied := *m
			out.Maintainers[i] = &copied
		}
	}
	out.Dependencies = copyDependencies(md.Dependencies)
	return &out
}

func copyLock(lock *chart.Lock) *chart.Lock {
	if lock == nil {
		return nil
	}
	out := *lock
	out.Dependencies = copyDependencies(lock.Dependencies)
	return &out
}

func copyDependencies(deps []*chart.Dependency) []*chart.Dependency {
	if deps == nil {
		return nil
	}
	out := make([]*chart.Dependency, len(deps))
	for i, dep := range deps {
		if dep == nil {
			continue
		}
		copied := *dep
		copied.Tags = slices.Clone(dep.Tags)
		copied.ImportValues = copyValueSlice(dep.ImportValues)
		out[i] = &copied
	}
	return out
}

func copyFiles(files []*common.File) []*common.File {
	if files == nil {
		return nil
	}
	out := make([]*common.File, len(files))
	for i, file := range files {
		if file == nil {
			continue
		}
		copied := *file
		copied.Data = bytes.Clone(file.Data)
		out[i] = &copied
	}
	return out
}

// copyValues deep copies the maps and slices that make up decoded chart values
// and release config. Other values are copied as interface values, which is
// enough for the scalars these maps normally hold.
func copyValues(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = copyValue(value)
	}
	return out
}

func copyValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return copyValues(typed)
	case map[any]any:
		out := make(map[any]any, len(typed))
		for key, nested := range typed {
			out[key] = copyValue(nested)
		}
		return out
	case []any:
		return copyValueSlice(typed)
	default:
		return value
	}
}

func copyValueSlice(values []any) []any {
	if values == nil {
		return nil
	}
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = copyValue(value)
	}
	return out
}
