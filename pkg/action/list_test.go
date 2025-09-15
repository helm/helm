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
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage"
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
	t.Helper()
	return NewList(actionConfigFixture(t))
}

func TestList_OneNamespace(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	makeMeSomeReleases(t, lister.cfg.Releases)
	list, err := lister.Run()
	is.NoError(err)
	is.Len(list, 3)
}

func TestList_AllNamespaces(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	makeMeSomeReleases(t, lister.cfg.Releases)
	lister.AllNamespaces = true
	lister.SetStateMask()
	list, err := lister.Run()
	is.NoError(err)
	is.Len(list, 3)
}

func TestList_Sort(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	lister.Sort = ByNameDesc // Other sorts are tested elsewhere
	makeMeSomeReleases(t, lister.cfg.Releases)
	list, err := lister.Run()
	is.NoError(err)
	is.Len(list, 3)
	is.Equal("two", list[0].Name)
	is.Equal("three", list[1].Name)
	is.Equal("one", list[2].Name)
}

func TestList_Limit(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	lister.Limit = 2
	makeMeSomeReleases(t, lister.cfg.Releases)
	list, err := lister.Run()
	is.NoError(err)
	is.Len(list, 2)
	// Lex order means one, three, two
	is.Equal("one", list[0].Name)
	is.Equal("three", list[1].Name)
}

func TestList_BigLimit(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	lister.Limit = 20
	makeMeSomeReleases(t, lister.cfg.Releases)
	list, err := lister.Run()
	is.NoError(err)
	is.Len(list, 3)

	// Lex order means one, three, two
	is.Equal("one", list[0].Name)
	is.Equal("three", list[1].Name)
	is.Equal("two", list[2].Name)
}

func TestList_LimitOffset(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	lister.Limit = 2
	lister.Offset = 1
	makeMeSomeReleases(t, lister.cfg.Releases)
	list, err := lister.Run()
	is.NoError(err)
	is.Len(list, 2)

	// Lex order means one, three, two
	is.Equal("three", list[0].Name)
	is.Equal("two", list[1].Name)
}

func TestList_LimitOffsetOutOfBounds(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	lister.Limit = 2
	lister.Offset = 3 // Last item is index 2
	makeMeSomeReleases(t, lister.cfg.Releases)
	list, err := lister.Run()
	is.NoError(err)
	is.Len(list, 0)

	lister.Limit = 10
	lister.Offset = 1
	list, err = lister.Run()
	is.NoError(err)
	is.Len(list, 2)
}

func TestList_StateMask(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	makeMeSomeReleases(t, lister.cfg.Releases)
	one, err := lister.cfg.Releases.Get("one", 1)
	is.NoError(err)
	one.SetStatus(release.StatusUninstalled, "uninstalled")
	err = lister.cfg.Releases.Update(one)
	is.NoError(err)

	// With the new default (ListAll), we should see all 3 releases by default
	res, err := lister.Run()
	is.NoError(err)
	is.Len(res, 3)
	is.Equal("one", res[0].Name)
	is.Equal("three", res[1].Name)

	lister.StateMask = ListUninstalled
	res, err = lister.Run()
	is.NoError(err)
	is.Len(res, 1)
	is.Equal("one", res[0].Name)

	lister.StateMask |= ListDeployed
	res, err = lister.Run()
	is.NoError(err)
	is.Len(res, 3)
}

func TestList_StateMaskWithStaleRevisions(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	lister.StateMask = ListFailed

	makeMeSomeReleasesWithStaleFailure(t, lister.cfg.Releases)

	res, err := lister.Run()

	is.NoError(err)
	is.Len(res, 1)

	// "dirty" release should _not_ be present as most recent
	// release is deployed despite failed release in past
	is.Equal("failed", res[0].Name)
}

func makeMeSomeReleasesWithStaleFailure(t *testing.T, store *storage.Storage) {
	t.Helper()
	one := namedReleaseStub("clean", release.StatusDeployed)
	one.Namespace = "default"
	one.Version = 1

	two := namedReleaseStub("dirty", release.StatusDeployed)
	two.Namespace = "default"
	two.Version = 1

	three := namedReleaseStub("dirty", release.StatusFailed)
	three.Namespace = "default"
	three.Version = 2

	four := namedReleaseStub("dirty", release.StatusDeployed)
	four.Namespace = "default"
	four.Version = 3

	five := namedReleaseStub("failed", release.StatusFailed)
	five.Namespace = "default"
	five.Version = 1

	for _, rel := range []*release.Release{one, two, three, four, five} {
		if err := store.Create(rel); err != nil {
			t.Fatal(err)
		}
	}

	all, err := store.ListReleases()
	assert.NoError(t, err)
	assert.Len(t, all, 5, "sanity test: five items added")
}

func TestList_Filter(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	lister.Filter = "th."
	makeMeSomeReleases(t, lister.cfg.Releases)

	res, err := lister.Run()
	is.NoError(err)
	is.Len(res, 1)
	is.Equal("three", res[0].Name)
}

func TestList_FilterFailsCompile(t *testing.T) {
	is := assert.New(t)
	lister := newListFixture(t)
	lister.Filter = "t[h.{{{"
	makeMeSomeReleases(t, lister.cfg.Releases)

	_, err := lister.Run()
	is.Error(err)
}

func makeMeSomeReleases(t *testing.T, store *storage.Storage) {
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

	for _, rel := range []*release.Release{one, two, three} {
		if err := store.Create(rel); err != nil {
			t.Fatal(err)
		}
	}

	all, err := store.ListReleases()
	assert.NoError(t, err)
	assert.Len(t, all, 3, "sanity test: three items added")
}

func TestFilterLatestReleases(t *testing.T) {
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

		filteredList := filterLatestReleases([]*release.Release{r1, r2, another})
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

		filteredList := filterLatestReleases([]*release.Release{r1, r2})
		expectedFilteredList := []*release.Release{r1, r2}

		assert.ElementsMatch(t, expectedFilteredList, filteredList)
	})
}

func TestSelectorList(t *testing.T) {
	r1 := releaseStub()
	r1.Name = "r1"
	r1.Version = 1
	r1.Labels = map[string]string{"key": "value1"}
	r2 := releaseStub()
	r2.Name = "r2"
	r2.Version = 1
	r2.Labels = map[string]string{"key": "value2"}
	r3 := releaseStub()
	r3.Name = "r3"
	r3.Version = 1
	r3.Labels = map[string]string{}

	lister := newListFixture(t)
	for _, rel := range []*release.Release{r1, r2, r3} {
		if err := lister.cfg.Releases.Create(rel); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("should fail selector parsing", func(t *testing.T) {
		is := assert.New(t)
		lister.Selector = "a?=b"

		_, err := lister.Run()
		is.Error(err)
	})

	t.Run("should select one release with matching label", func(t *testing.T) {
		lister.Selector = "key==value1"
		res, _ := lister.Run()

		expectedFilteredList := []*release.Release{r1}
		assert.ElementsMatch(t, expectedFilteredList, res)
	})

	t.Run("should select two releases with non matching label", func(t *testing.T) {
		lister.Selector = "key!=value1"
		res, _ := lister.Run()

		expectedFilteredList := []*release.Release{r2, r3}
		assert.ElementsMatch(t, expectedFilteredList, res)
	})
}

func TestListRun_UnreachableKubeClient(t *testing.T) {
	config := actionConfigFixture(t)
	failingKubeClient := kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: nil}
	failingKubeClient.ConnectionError = errors.New("connection refused")
	config.KubeClient = &failingKubeClient

	lister := NewList(config)
	result, err := lister.Run()

	assert.Nil(t, result)
	assert.ErrorContains(t, err, "connection refused")
}
