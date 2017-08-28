/*
Copyright 2017 The Kubernetes Authors All rights reserved.
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

package e2e

import (
	"flag"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var url string
var tillerHost string
var helmBinPath string
var localTiller bool

func init() {
	flag.StringVar(&url, "cluster-url", "http://127.0.0.1:8080", "apiserver address to use with restclient")
	flag.StringVar(&tillerHost, "tiller-host", "", "tiller address")
	flag.StringVar(&helmBinPath, "helm-bin", "helm", "helm binary to test")
	flag.BoolVar(&localTiller, "local-tiller", false, "wait for tiller pod")
}

func LoadConfig() *rest.Config {
	config, err := clientcmd.BuildConfigFromFlags(url, "")
	Expect(err).NotTo(HaveOccurred())
	return config
}

func KubeClient() (*kubernetes.Clientset, error) {
	config := LoadConfig()
	clientset, err := kubernetes.NewForConfig(config)
	Expect(err).NotTo(HaveOccurred())
	return clientset, nil
}

func DeleteNS(clientset kubernetes.Interface, namespace *v1.Namespace) {
	defer GinkgoRecover()
	err := clientset.Core().Namespaces().Delete(namespace.Name, nil)
	Expect(err).NotTo(HaveOccurred())
}

func Logf(format string, a ...interface{}) {
	fmt.Fprintf(GinkgoWriter, format, a...)
}

func WaitForPod(clientset kubernetes.Interface, namespace string, name string, phase v1.PodPhase) *v1.Pod {
	defer GinkgoRecover()
	var podUpdated *v1.Pod
	Eventually(func() error {
		podUpdated, err := clientset.Core().Pods(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if phase != "" && podUpdated.Status.Phase != phase {
			return fmt.Errorf("pod %v is not %v phase: %v", podUpdated.Name, phase, podUpdated.Status.Phase)
		}
		return nil
	}, 1*time.Minute, 3*time.Second).Should(BeNil())
	return podUpdated
}
