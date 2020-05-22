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

package action

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage"
)

func TestListStates(t *testing.T) {
	for input, expect := range map[string]ListStates{
		"deployed":            ListDeployed,
		"uninstalled":         ListUninstalled,
		"uninstalling":        ListUninstalling,
		"superseded":          ListSuperseded,
		"failed":              ListFailed,
		"pending-install":     ListPendingInstall,
		"pending-rollback":    ListPendingRollback,
		"pending-upgrade":     ListPendingUpgrade,
		"unknown":             ListUnknown,
		"totally made up key": ListUnknown,
	} {
		if expect != expect.FromName(input) {
			t.Errorf("Expected %d for %s", expect, input)
		}
		// This is a cheap way to verify that ListAll actually allows everything but Unknown
		if got := expect.FromName(input); got != ListUnknown && got&ListAll == 0 {
			t.Errorf("Expected %s to match the ListAll filter", input)
		}
	}

	filter := ListDeployed | ListPendingRollback
	if status := filter.FromName("deployed"); filter&status == 0 {
		t.Errorf("Expected %d to match mask %d", status, filter)
	}
	if status := filter.FromName("failed"); filter&status != 0 {
		t.Errorf("Expected %d to fail to match mask %d", status, filter)
	}
}

func TestList_Empty(t *testing.T) {
	lister := NewList(actionConfigFixture(t))
	list, err := lister.Run()
	assert.NoError(t, err)
	assert.Len(t, list, 0)
}

func newListFixture(t *testing.T) *List {
	return NewList(actionConfigFixture(t))
}

func TestList_OneNamespace(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	makeMeSomeReleases(lister.cfg.Releases, t)
	list, err := lister.Run()
	is.NoError(err)
	is.Len(list, 4)
}

func TestList_AllNamespaces(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	makeMeSomeReleases(lister.cfg.Releases, t)
	lister.AllNamespaces = true
	lister.SetStateMask()
	list, err := lister.Run()
	is.NoError(err)
	is.Len(list, 4)
}

func TestList_Deployed(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	makeMeSomeReleases(lister.cfg.Releases, t)
	lister.Deployed = true
	lister.SetStateMask()
	list, err := lister.Run()
	is.NoError(err)
	is.Len(list, 3)
}

func TestList_Uninstalled(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	makeMeSomeReleases(lister.cfg.Releases, t)
	lister.Uninstalled = true
	lister.SetStateMask()
	list, err := lister.Run()
	is.NoError(err)
	is.Len(list, 1)
}

func TestList_Uninstalling(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	makeMeSomeReleases(lister.cfg.Releases, t)
	lister.Uninstalling = true
	lister.SetStateMask()
	list, err := lister.Run()
	is.NoError(err)
	is.Len(list, 1)
}

func TestList_Pending(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	makeMeSomeReleases(lister.cfg.Releases, t)
	lister.Pending = true
	lister.SetStateMask()
	list, err := lister.Run()
	is.NoError(err)
	is.Len(list, 1)
}

func TestList_Failed(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	makeMeSomeReleases(lister.cfg.Releases, t)
	lister.Failed = true
	lister.SetStateMask()
	list, err := lister.Run()
	is.NoError(err)
	is.Len(list, 1)
}

func TestList_Superseded(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	makeMeSomeReleases(lister.cfg.Releases, t)
	lister.Superseded = true
	lister.SetStateMask()
	list, err := lister.Run()
	is.NoError(err)
	is.Len(list, 1)
}

func TestList_Sort(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	lister.Sort = ByNameDesc // Other sorts are tested elsewhere
	makeMeSomeReleases(lister.cfg.Releases, t)
	list, err := lister.Run()
	is.NoError(err)
	is.Len(list, 4)
	is.Equal("two", list[0].Name)
	is.Equal("three", list[1].Name)
	is.Equal("seven", list[2].Name)
	is.Equal("one", list[3].Name)
}

func TestList_Limit(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	lister.Limit = 2
	makeMeSomeReleases(lister.cfg.Releases, t)
	list, err := lister.Run()
	is.NoError(err)
	is.Len(list, 2)
	// Lex order means one, seven, three, two
	is.Equal("one", list[0].Name)
	is.Equal("seven", list[1].Name)
}

func TestList_BigLimit(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	lister.Limit = 20
	makeMeSomeReleases(lister.cfg.Releases, t)
	list, err := lister.Run()
	is.NoError(err)
	is.Len(list, 4)

	// Lex order means one, seven, three, two
	is.Equal("one", list[0].Name)
	is.Equal("seven", list[1].Name)
	is.Equal("three", list[2].Name)
	is.Equal("two", list[3].Name)
}

func TestList_LimitOffset(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	lister.Limit = 2
	lister.Offset = 1
	makeMeSomeReleases(lister.cfg.Releases, t)
	list, err := lister.Run()
	is.NoError(err)
	is.Len(list, 2)

	// Lex order means one, seven, three, two
	is.Equal("seven", list[0].Name)
	is.Equal("three", list[1].Name)
}

func TestList_LimitOffsetOutOfBounds(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	lister.Limit = 2
	lister.Offset = 4 // Last item is index 2
	makeMeSomeReleases(lister.cfg.Releases, t)
	list, err := lister.Run()
	is.NoError(err)
	is.Len(list, 0)

	lister.Limit = 10
	lister.Offset = 1
	list, err = lister.Run()
	is.NoError(err)
	is.Len(list, 3)
}

func TestList_StateMask(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	makeMeSomeReleases(lister.cfg.Releases, t)
	one, err := lister.cfg.Releases.Get("one", 1)
	is.NoError(err)
	one.SetStatus(release.StatusUninstalled, "uninstalled")
	err = lister.cfg.Releases.Update(one)
	is.NoError(err)

	res, err := lister.Run()
	is.NoError(err)
	is.Len(res, 3)
	is.Equal("seven", res[0].Name)
	is.Equal("three", res[1].Name)

	lister.StateMask = ListUninstalled
	res, err = lister.Run()
	is.NoError(err)
	is.Len(res, 2)
	is.Equal("four", res[0].Name)

	lister.StateMask |= ListDeployed
	res, err = lister.Run()
	is.NoError(err)
	is.Len(res, 4)
}

func TestList_Filter(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	lister.Filter = "th."
	makeMeSomeReleases(lister.cfg.Releases, t)

	res, err := lister.Run()
	is.NoError(err)
	is.Len(res, 1)
	is.Equal("three", res[0].Name)
}

func TestList_FilterFailsCompile(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	lister.Filter = "t[h.{{{"
	makeMeSomeReleases(lister.cfg.Releases, t)

	_, err := lister.Run()
	is.Error(err)
}

func makeMeSomeReleases(store *storage.Storage, t *testing.T) {
	t.Helper()
	one := releaseStub()
	one.Name = "one"
	one.Namespace = "default"
	one.Version = 1
	two := releaseStub()
	two.Name = "two"
	two.Namespace = "default"
	two.Version = 2
	three := releaseStub()
	three.Name = "three"
	three.Namespace = "default"
	three.Version = 3
	four := namedReleaseStub("prims47-fuego", release.StatusUninstalled)
	four.Name = "four"
	four.Namespace = "default"
	four.Version = 1
	five := namedReleaseStub("prims47-benArfa", release.StatusUninstalling)
	five.Name = "five"
	five.Namespace = "default"
	five.Version = 1
	six := namedReleaseStub("prims47-benArfa", release.StatusPendingInstall)
	six.Name = "six"
	six.Namespace = "default"
	six.Version = 1
	seven := namedReleaseStub("prims47-benArfa", release.StatusFailed)
	seven.Name = "seven"
	seven.Namespace = "default"
	seven.Version = 1
	eight := namedReleaseStub("prims47-benArfa", release.StatusSuperseded)
	eight.Name = "eight"
	eight.Namespace = "default"
	eight.Version = 1

	for _, rel := range []*release.Release{one, two, three, four, five, six, seven, eight} {
		if err := store.Create(rel); err != nil {
			t.Fatal(err)
		}
	}

	all, err := store.ListReleases()
	assert.NoError(t, err)
	assert.Len(t, all, 8, "sanity test: three items added")
}

func TestFilterList(t *testing.T) {
	t.Run("should filter old versions of the same release", func(t *testing.T) {
		r1 := releaseStub()
		r1.Name = "r"
		r1.Version = 1
		r2 := releaseStub()
		r2.Name = "r"
		r2.Version = 2
		another := releaseStub()
		another.Name = "another"
		another.Version = 1

		filteredList := filterList([]*release.Release{r1, r2, another})
		expectedFilteredList := []*release.Release{r2, another}

		assert.ElementsMatch(t, expectedFilteredList, filteredList)
	})

	t.Run("should not filter out any version across namespaces", func(t *testing.T) {
		r1 := releaseStub()
		r1.Name = "r"
		r1.Namespace = "default"
		r1.Version = 1
		r2 := releaseStub()
		r2.Name = "r"
		r2.Namespace = "testing"
		r2.Version = 2

		filteredList := filterList([]*release.Release{r1, r2})
		expectedFilteredList := []*release.Release{r1, r2}

		assert.ElementsMatch(t, expectedFilteredList, filteredList)
	})
}
