package engine

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var config *rest.Config

func init() {
	// try the out-cluster config, this will default to the in-cluster config is not successful
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config1, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err == nil {
		config = config1
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func lookup(apiversion string, resource string, namespace string, name string) (map[string]interface{}, error) {
	var client dynamic.ResourceInterface
	c, namespaced, err := getDynamicClientOnKind(apiversion, resource)
	if err != nil {
		return map[string]interface{}{}, err
	}
	if namespaced && namespace != "" {
		client = c.Namespace(namespace)
	} else {
		client = c
	}
	if name != "" {
		//this will return a single object
		obj, err := client.Get(name, metav1.GetOptions{})
		if err != nil {
			return map[string]interface{}{}, err
		}
		return obj.Object, nil
	}
	//this will return a list
	obj, err := client.List(metav1.ListOptions{})
	if err != nil {
		return map[string]interface{}{}, err
	}
	return obj.Object, nil

}

// GetDynamicClientOnUnstructured returns a dynamic client on an Unstructured type. This client can be further namespaced.
func getDynamicClientOnKind(apiversion string, kind string) (dynamic.NamespaceableResourceInterface, bool, error) {
	gvk := schema.FromAPIVersionAndKind(apiversion, kind)
	apiRes, err := getAPIReourceForGVK(gvk)
	if err != nil {
		log.Printf("[ERROR] unable to get apiresource from unstructured: %s , error %s", gvk.String(), err)
		return nil, false, err
	}
	gvr := schema.GroupVersionResource{
		Group:    apiRes.Group,
		Version:  apiRes.Version,
		Resource: apiRes.Name,
	}
	intf, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Printf("[ERROR] unable to get dynamic client %s", err)
		return nil, false, err
	}
	res := intf.Resource(gvr)
	return res, apiRes.Namespaced, nil
}

func getAPIReourceForGVK(gvk schema.GroupVersionKind) (metav1.APIResource, error) {
	res := metav1.APIResource{}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		log.Printf("[ERROR] unable to create discovery client %s", err)
		return res, err
	}
	resList, err := discoveryClient.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		log.Printf("[ERROR] unable to retrieve resource list for: %s , error: %s", gvk.GroupVersion().String(), err)
		return res, err
	}
	for _, resource := range resList.APIResources {
		if resource.Kind == gvk.Kind && !strings.Contains(resource.Name, "/") {
			res = resource
			res.Group = gvk.Group
			res.Version = gvk.Version
			break
		}
	}
	return res, nil
}
