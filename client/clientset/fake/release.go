package fake

import (
	aci "k8s.io/helm/api"
	"k8s.io/kubernetes/pkg/api"
	schema "k8s.io/kubernetes/pkg/api/unversioned"
	testing "k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"
)

type FakeRelease struct {
	Fake *testing.Fake
	ns   string
}

var certResource = schema.GroupVersionResource{Group: "helm.sh", Version: "v1beta1", Resource: "releases"}

// Get returns the Release by name.
func (mock *FakeRelease) Get(name string) (*aci.Release, error) {
	obj, err := mock.Fake.
		Invokes(testing.NewGetAction(certResource, mock.ns, name), &aci.Release{})

	if obj == nil {
		return nil, err
	}
	return obj.(*aci.Release), err
}

// List returns the a of Releases.
func (mock *FakeRelease) List(opts api.ListOptions) (*aci.ReleaseList, error) {
	obj, err := mock.Fake.
		Invokes(testing.NewListAction(certResource, mock.ns, opts), &aci.Release{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &aci.ReleaseList{}
	for _, item := range obj.(*aci.ReleaseList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Create creates a new Release.
func (mock *FakeRelease) Create(svc *aci.Release) (*aci.Release, error) {
	obj, err := mock.Fake.
		Invokes(testing.NewCreateAction(certResource, mock.ns, svc), &aci.Release{})

	if obj == nil {
		return nil, err
	}
	return obj.(*aci.Release), err
}

// Update updates a Release.
func (mock *FakeRelease) Update(svc *aci.Release) (*aci.Release, error) {
	obj, err := mock.Fake.
		Invokes(testing.NewUpdateAction(certResource, mock.ns, svc), &aci.Release{})

	if obj == nil {
		return nil, err
	}
	return obj.(*aci.Release), err
}

// Delete deletes a Release by name.
func (mock *FakeRelease) Delete(name string) error {
	_, err := mock.Fake.
		Invokes(testing.NewDeleteAction(certResource, mock.ns, name), &aci.Release{})

	return err
}

func (mock *FakeRelease) UpdateStatus(srv *aci.Release) (*aci.Release, error) {
	obj, err := mock.Fake.
		Invokes(testing.NewUpdateSubresourceAction(certResource, "status", mock.ns, srv), &aci.Release{})

	if obj == nil {
		return nil, err
	}
	return obj.(*aci.Release), err
}

func (mock *FakeRelease) Watch(opts api.ListOptions) (watch.Interface, error) {
	return mock.Fake.
		InvokesWatch(testing.NewWatchAction(certResource, mock.ns, opts))
}
