package kube

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

// this file is mostly copied from the "[kubectl client-side apply]" migration
// code with a few small modifications:
// 1) we do not special-case subresources or apply type for other field managers; helm assumes that it has sole control over the 'managersToAdopt'
// 2) we automatically update previous 'Update' calls from helm to now be 'Apply' calls.
//
// [kubectl client-side apply]: https://github.com/kubernetes/kubernetes/blob/a4b8a3b2e33a3b591884f69b64f439e6b880dc40/staging/src/k8s.io/client-go/util/csaupgrade/upgrade.go#L122
func migrateManagedFields(
	helper *resource.Helper,
	info *resource.Info,
	managersToAdopt []string,
	helmManager string,
) (didMigrate bool, err error) {
	// retry a few times on conflict errors.
	for i := 0; i < 5; i++ {
		var patchData []byte
		var obj runtime.Object

		patchData, err := createMigrateManagedFieldsPatch(info.Object, managersToAdopt, helmManager)
		if err != nil {
			return false, errors.Wrap(err, "failed to generate patch for upgrading managed fields")
		} else if patchData == nil {
			// no work to do.
			return false, nil
		}

		obj, err = helper.Patch(info.Namespace, info.Name, types.JSONPatchType, patchData, nil)
		if err != nil {
			if !apierrors.IsConflict(err) {
				return false, errors.Wrap(err, "unexpected error patching managed fields on object")
			}
			// retry on conflicts, but refresh object first
			if err = info.Get(); err != nil {
				return false, errors.Wrap(err, "unexpected error refreshing object")
			}
			continue
		}
		info.Refresh(obj, true)
		return true, nil
	}
	return false, nil
}

// createMigrateManagedFieldsPatch Calculates a minimal JSON Patch to send to upgrade managed fields
func createMigrateManagedFieldsPatch(
	obj runtime.Object,
	managersToAdopt []string,
	helmManager string,
) ([]byte, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	managedFields := accessor.GetManagedFields()
	filteredManagers := accessor.GetManagedFields()
	// note that we also adopt previous non-apply operations from the helm manager.
	for managerName := range sets.New(managersToAdopt...).Insert(helmManager) {
		filteredManagers, err = upgradedManagedFields(
			filteredManagers,
			managerName,
			helmManager,
		)
		if err != nil {
			return nil, err
		}
	}

	if reflect.DeepEqual(managedFields, filteredManagers) {
		// If the managed fields have not changed from the transformed version,
		// there is no patch to perform
		return nil, nil
	}

	// Create a patch with a diff between old and new objects.
	// Just include all managed fields since that is only thing that will change
	//
	// Also include test for RV to avoid race condition
	jsonPatch := []map[string]interface{}{
		{
			"op":    "replace",
			"path":  "/metadata/managedFields",
			"value": filteredManagers,
		},
		{
			// Use "replace" instead of "test" operation so that etcd rejects with
			// 409 conflict instead of apiserver with an invalid request
			"op":    "replace",
			"path":  "/metadata/resourceVersion",
			"value": accessor.GetResourceVersion(),
		},
	}

	return json.Marshal(jsonPatch)
}

// Returns a copy of the provided managed fields that has been migrated from
// client-side-apply to server-side-apply, or an error if there was an issue
func upgradedManagedFields(
	managedFields []metav1.ManagedFieldsEntry,
	oldManager,
	newManager string,
) ([]metav1.ManagedFieldsEntry, error) {
	if managedFields == nil {
		return nil, nil
	}

	// Create managed fields clone since we modify the values
	managedFieldsCopy := make([]metav1.ManagedFieldsEntry, len(managedFields))
	if copy(managedFieldsCopy, managedFields) != len(managedFields) {
		return nil, errors.New("failed to copy managed fields")
	}
	managedFields = managedFieldsCopy

	// Locate new manager
	replaceIndex, ok := findFirstIndex(managedFields,
		func(entry metav1.ManagedFieldsEntry) bool {
			return entry.Manager == newManager &&
				entry.Operation == metav1.ManagedFieldsOperationApply
		})

	if !ok {
		return nil, errors.New("apply: unexpected error - no manager found")
	}
	err := unionManagerIntoIndex(managedFields, replaceIndex, oldManager)
	if err != nil {
		return nil, err
	}

	// Create version of managed fields without the old field manager.
	filteredManagers := filter(managedFields, func(entry metav1.ManagedFieldsEntry) bool {
		if entry.Manager != oldManager {
			// keep unaffected entries
			return true
		} else if oldManager != newManager {
			// remove if a different field manager entirely.
			return false
		}
		// special-case: if migrating the same field manager, only remove the old non-Apply entries.
		return (entry.Manager == newManager && entry.Operation == metav1.ManagedFieldsOperationApply)
	})

	return filteredManagers, nil
}

func unionManagerIntoIndex(
	entries []metav1.ManagedFieldsEntry,
	targetIndex int,
	oldManager string,
) error {
	ssaManager := entries[targetIndex]

	// find any other manager of same APIVersion, union ssa fields with it.
	oldManagerIndex, ok := findFirstIndex(entries,
		func(entry metav1.ManagedFieldsEntry) bool {
			return entry.Manager == oldManager &&
				entry.Operation == metav1.ManagedFieldsOperationUpdate &&
				entry.APIVersion == ssaManager.APIVersion
		})

	targetFieldSet, err := decodeManagedFieldsEntrySet(ssaManager)
	if err != nil {
		return fmt.Errorf("failed to convert fields to set: %w", err)
	}

	combinedFieldSet := &targetFieldSet

	// Union the old manager with the new manager. Do nothing if
	// there was no good candidate found
	if ok {
		csaManager := entries[oldManagerIndex]

		csaFieldSet, err := decodeManagedFieldsEntrySet(csaManager)
		if err != nil {
			return fmt.Errorf("failed to convert fields to set: %w", err)
		}

		combinedFieldSet = combinedFieldSet.Union(&csaFieldSet)
	}

	// Encode the fields back to the serialized format
	err = encodeManagedFieldsEntrySet(&entries[targetIndex], *combinedFieldSet)
	if err != nil {
		return fmt.Errorf("failed to encode field set: %w", err)
	}

	return nil
}

func findFirstIndex[T any](
	collection []T,
	predicate func(T) bool,
) (int, bool) {
	for idx, entry := range collection {
		if predicate(entry) {
			return idx, true
		}
	}

	return -1, false
}

func filter[T any](
	collection []T,
	predicate func(T) bool,
) []T {
	result := make([]T, 0, len(collection))

	for _, value := range collection {
		if predicate(value) {
			result = append(result, value)
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

// Included from fieldmanager.internal to avoid dependency cycle
// FieldsToSet creates a set paths from an input trie of fields
func decodeManagedFieldsEntrySet(f metav1.ManagedFieldsEntry) (s fieldpath.Set, err error) {
	err = s.FromJSON(bytes.NewReader(f.FieldsV1.Raw))
	return s, err
}

// SetToFields creates a trie of fields from an input set of paths
func encodeManagedFieldsEntrySet(f *metav1.ManagedFieldsEntry, s fieldpath.Set) (err error) {
	f.FieldsV1.Raw, err = s.ToJSON()
	return err
}
