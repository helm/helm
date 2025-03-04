/*
Copyright 2023 The Kubernetes Authors.

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

package client

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

/*
Adapted from controller-runtime v0.19 before The Kubernetes Aggregated Discovery was enabled
in controller-runtime v0.20 which broke the preferred version discovery in the RESTMapper.
https://github.com/kubernetes-sigs/controller-runtime/blob/e818ce450d3d358600848dcfa1b585de64e7c865/pkg/client/apiutil/restmapper.go
*/

// NewLazyRESTMapper returns a dynamic RESTMapper for cfg. The dynamic
// RESTMapper dynamically discovers resource types at runtime.
func NewLazyRESTMapper(cfg *rest.Config, httpClient *http.Client) (meta.RESTMapper, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("httpClient must not be nil, consider using rest.HTTPClientFor(c) to create a client")
	}

	client, err := discovery.NewDiscoveryClientForConfigAndClient(cfg, httpClient)
	if err != nil {
		return nil, err
	}
	return &mapper{
		mapper:      restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{}),
		client:      client,
		knownGroups: map[string]*restmapper.APIGroupResources{},
		apiGroups:   map[string]*metav1.APIGroup{},
	}, nil
}

// mapper is a RESTMapper that will lazily query the provided
// client for discovery information to do REST mappings.
type mapper struct {
	mapper      meta.RESTMapper
	client      discovery.DiscoveryInterface
	knownGroups map[string]*restmapper.APIGroupResources
	apiGroups   map[string]*metav1.APIGroup

	// mutex to provide thread-safe mapper reloading.
	mu sync.RWMutex
}

// KindFor implements Mapper.KindFor.
func (m *mapper) KindFor(resource schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	res, err := m.getMapper().KindFor(resource)
	if meta.IsNoMatchError(err) {
		if err := m.addKnownGroupAndReload(resource.Group, resource.Version); err != nil {
			return schema.GroupVersionKind{}, err
		}
		res, err = m.getMapper().KindFor(resource)
	}

	return res, err
}

// KindsFor implements Mapper.KindsFor.
func (m *mapper) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	res, err := m.getMapper().KindsFor(resource)
	if meta.IsNoMatchError(err) {
		if err := m.addKnownGroupAndReload(resource.Group, resource.Version); err != nil {
			return nil, err
		}
		res, err = m.getMapper().KindsFor(resource)
	}

	return res, err
}

// ResourceFor implements Mapper.ResourceFor.
func (m *mapper) ResourceFor(input schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	res, err := m.getMapper().ResourceFor(input)
	if meta.IsNoMatchError(err) {
		if err := m.addKnownGroupAndReload(input.Group, input.Version); err != nil {
			return schema.GroupVersionResource{}, err
		}
		res, err = m.getMapper().ResourceFor(input)
	}

	return res, err
}

// ResourcesFor implements Mapper.ResourcesFor.
func (m *mapper) ResourcesFor(input schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	res, err := m.getMapper().ResourcesFor(input)
	if meta.IsNoMatchError(err) {
		if err := m.addKnownGroupAndReload(input.Group, input.Version); err != nil {
			return nil, err
		}
		res, err = m.getMapper().ResourcesFor(input)
	}

	return res, err
}

// RESTMapping implements Mapper.RESTMapping.
func (m *mapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	res, err := m.getMapper().RESTMapping(gk, versions...)
	if meta.IsNoMatchError(err) {
		if err := m.addKnownGroupAndReload(gk.Group, versions...); err != nil {
			return nil, err
		}
		res, err = m.getMapper().RESTMapping(gk, versions...)
	}

	return res, err
}

// RESTMappings implements Mapper.RESTMappings.
func (m *mapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	res, err := m.getMapper().RESTMappings(gk, versions...)
	if meta.IsNoMatchError(err) {
		if err := m.addKnownGroupAndReload(gk.Group, versions...); err != nil {
			return nil, err
		}
		res, err = m.getMapper().RESTMappings(gk, versions...)
	}

	return res, err
}

// ResourceSingularizer implements Mapper.ResourceSingularizer.
func (m *mapper) ResourceSingularizer(resource string) (string, error) {
	return m.getMapper().ResourceSingularizer(resource)
}

func (m *mapper) getMapper() meta.RESTMapper {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mapper
}

// addKnownGroupAndReload reloads the mapper with updated information about missing API group.
// versions can be specified for partial updates, for instance for v1beta1 version only.
func (m *mapper) addKnownGroupAndReload(groupName string, versions ...string) error {
	// versions will here be [""] if the forwarded Version value of
	// GroupVersionResource (in calling method) was not specified.
	if len(versions) == 1 && versions[0] == "" {
		versions = nil
	}

	// If no specific versions are set by user, we will scan all available ones for the API group.
	// This operation requires 2 requests: /api and /apis, but only once. For all subsequent calls
	// this data will be taken from cache.
	if len(versions) == 0 {
		apiGroup, err := m.findAPIGroupByName(groupName)
		if err != nil {
			return err
		}
		if apiGroup != nil {
			for _, version := range apiGroup.Versions {
				versions = append(versions, version.Version)
			}
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Create or fetch group resources from cache.
	groupResources := &restmapper.APIGroupResources{
		Group:              metav1.APIGroup{Name: groupName},
		VersionedResources: make(map[string][]metav1.APIResource),
	}

	// Update information for group resources about versioned resources.
	// The number of API calls is equal to the number of versions: /apis/<group>/<version>.
	// If we encounter a missing API version (NotFound error), we will remove the group from
	// the m.apiGroups and m.knownGroups caches.
	// If this happens, in the next call the group will be added back to apiGroups
	// and only the existing versions will be loaded in knownGroups.
	groupVersionResources, err := m.fetchGroupVersionResourcesLocked(groupName, versions...)
	if err != nil {
		return fmt.Errorf("failed to get API group resources: %w", err)
	}

	if _, ok := m.knownGroups[groupName]; ok {
		groupResources = m.knownGroups[groupName]
	}

	// Update information for group resources about the API group by adding new versions.
	// Ignore the versions that are already registered.
	for groupVersion, resources := range groupVersionResources {
		version := groupVersion.Version

		groupResources.VersionedResources[version] = resources.APIResources
		found := false
		for _, v := range groupResources.Group.Versions {
			if v.Version == version {
				found = true
				break
			}
		}

		if !found {
			groupResources.Group.Versions = append(groupResources.Group.Versions, metav1.GroupVersionForDiscovery{
				GroupVersion: metav1.GroupVersion{Group: groupName, Version: version}.String(),
				Version:      version,
			})
		}
	}

	// Update data in the cache.
	m.knownGroups[groupName] = groupResources

	// Finally, update the group with received information and regenerate the mapper.
	updatedGroupResources := make([]*restmapper.APIGroupResources, 0, len(m.knownGroups))
	for _, agr := range m.knownGroups {
		updatedGroupResources = append(updatedGroupResources, agr)
	}

	m.mapper = restmapper.NewDiscoveryRESTMapper(updatedGroupResources)
	return nil
}

// findAPIGroupByNameLocked returns API group by its name.
func (m *mapper) findAPIGroupByName(groupName string) (*metav1.APIGroup, error) {
	// Looking in the cache first.
	{
		m.mu.RLock()
		group, ok := m.apiGroups[groupName]
		m.mu.RUnlock()
		if ok {
			return group, nil
		}
	}

	// Update the cache if nothing was found.
	apiGroups, err := m.client.ServerGroups()
	if err != nil {
		return nil, fmt.Errorf("failed to get server groups: %w", err)
	}
	if len(apiGroups.Groups) == 0 {
		return nil, fmt.Errorf("received an empty API groups list")
	}

	m.mu.Lock()
	for i := range apiGroups.Groups {
		group := &apiGroups.Groups[i]
		m.apiGroups[group.Name] = group
	}
	m.mu.Unlock()

	// Looking in the cache again.
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Don't return an error here if the API group is not present.
	// The reloaded RESTMapper will take care of returning a NoMatchError.
	return m.apiGroups[groupName], nil
}

// fetchGroupVersionResourcesLocked fetches the resources for the specified group and its versions.
// This method might modify the cache so it needs to be called under the lock.
func (m *mapper) fetchGroupVersionResourcesLocked(groupName string, versions ...string) (map[schema.GroupVersion]*metav1.APIResourceList, error) {
	groupVersionResources := make(map[schema.GroupVersion]*metav1.APIResourceList)
	failedGroups := make(map[schema.GroupVersion]error)

	for _, version := range versions {
		groupVersion := schema.GroupVersion{Group: groupName, Version: version}

		apiResourceList, err := m.client.ServerResourcesForGroupVersion(groupVersion.String())
		if apierrors.IsNotFound(err) {
			// If the version is not found, we remove the group from the cache
			// so it gets refreshed on the next call.
			if m.isAPIGroupCached(groupVersion) {
				delete(m.apiGroups, groupName)
			}
			if m.isGroupVersionCached(groupVersion) {
				delete(m.knownGroups, groupName)
			}
			continue
		} else if err != nil {
			failedGroups[groupVersion] = err
		}

		if apiResourceList != nil {
			// even in case of error, some fallback might have been returned.
			groupVersionResources[groupVersion] = apiResourceList
		}
	}

	if len(failedGroups) > 0 {
		err := ErrResourceDiscoveryFailed(failedGroups)
		return nil, &err
	}

	return groupVersionResources, nil
}

// isGroupVersionCached checks if a version for a group is cached in the known groups cache.
func (m *mapper) isGroupVersionCached(gv schema.GroupVersion) bool {
	if cachedGroup, ok := m.knownGroups[gv.Group]; ok {
		_, cached := cachedGroup.VersionedResources[gv.Version]
		return cached
	}

	return false
}

// isAPIGroupCached checks if a version for a group is cached in the api groups cache.
func (m *mapper) isAPIGroupCached(gv schema.GroupVersion) bool {
	cachedGroup, ok := m.apiGroups[gv.Group]
	if !ok {
		return false
	}

	for _, version := range cachedGroup.Versions {
		if version.Version == gv.Version {
			return true
		}
	}

	return false
}

// ErrResourceDiscoveryFailed is returned if the RESTMapper cannot discover supported resources for some GroupVersions.
// It wraps the errors encountered, except "NotFound" errors are replaced with meta.NoResourceMatchError, for
// backwards compatibility with code that uses meta.IsNoMatchError() to check for unsupported APIs.
type ErrResourceDiscoveryFailed map[schema.GroupVersion]error

// Error implements the error interface.
func (e *ErrResourceDiscoveryFailed) Error() string {
	subErrors := []string{}
	for k, v := range *e {
		subErrors = append(subErrors, fmt.Sprintf("%s: %v", k, v))
	}
	sort.Strings(subErrors)
	return fmt.Sprintf("unable to retrieve the complete list of server APIs: %s", strings.Join(subErrors, ", "))
}

func (e *ErrResourceDiscoveryFailed) Unwrap() []error {
	subErrors := []error{}
	for gv, err := range *e {
		if apierrors.IsNotFound(err) {
			err = &meta.NoResourceMatchError{PartialResource: gv.WithResource("")}
		}
		subErrors = append(subErrors, err)
	}
	return subErrors
}
