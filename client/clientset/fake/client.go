package fake

import (
	_ "github.com/appscode/log"
	"k8s.io/kubernetes/pkg/client/clientset_generated/release_1_5/fake"
	"k8s.io/kubernetes/pkg/runtime"
)

type ClientSets struct {
	*fake.Clientset
	ExtensionClient *FakeExtensionClient
}

func NewFakeClient(objects ...runtime.Object) *ClientSets {
	return &ClientSets{
		Clientset:       fake.NewSimpleClientset(objects...),
		ExtensionClient: NewFakeExtensionClient(objects...),
	}
}
