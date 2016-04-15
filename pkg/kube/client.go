package kube

import (
	"fmt"
	"io"

	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
)

const includeThirdPartyAPIs = false

// ResourceActorFunc performs an action on a signle resource.
type ResourceActorFunc func(*resource.Info) error

// Create creates kubernetes resources from an io.reader
//
// Namespace will set the namespace
// Config allows for overiding values from kubectl
func Create(namespace string, reader io.Reader, config clientcmd.ClientConfig) error {
	return perform(namespace, reader, createResource, config)
}

func perform(namespace string, reader io.Reader, fn ResourceActorFunc, config clientcmd.ClientConfig) error {
	f := cmdutil.NewFactory(config)
	//schema, err := f.Validator(true, "")
	//if err != nil {
	//return err
	//}

	r := f.NewBuilder(includeThirdPartyAPIs).
		ContinueOnError().
		NamespaceParam(namespace).
		RequireNamespace().
		Stream(reader, "").
		Flatten().
		Do()

	if r.Err() != nil {
		return r.Err()
	}

	count := 0
	err := r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		err = fn(info)

		if err == nil {
			count++
		}
		return err
	})

	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no objects passed to create")
	}
	return nil
}

func createResource(info *resource.Info) error {
	_, err := resource.NewHelper(info.Client, info.Mapping).Create(info.Namespace, true, info.Object)
	return err
}
