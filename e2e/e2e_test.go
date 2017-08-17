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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
)

var _ = Describe("Basic Suite", func() {
	var helm HelmManager
	var namespace *v1.Namespace
	var clientset kubernetes.Interface

	BeforeEach(func() {
		var err error
		clientset, err = KubeClient()
		Expect(err).NotTo(HaveOccurred())
		By("Creating namespace and initializing test framework")
		namespaceObj := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "e2e-helm-",
			},
		}
		namespace, err = clientset.Core().Namespaces().Create(namespaceObj)
		Expect(err).NotTo(HaveOccurred())
		helm = &BinaryHelmManager{
			Namespace:  namespace.Name,
			Clientset:  clientset,
			HelmBin:    helmBinPath,
			TillerHost: tillerHost,
		}
		if !localTiller {
			Expect(helm.InstallTiller()).NotTo(HaveOccurred())
		}
	})

	AfterEach(func() {
		By("Removing namespace")
		DeleteNS(clientset, namespace)
	})

	It("Should be possible to create/delete/upgrade/rollback and check status of wordpress chart", func() {
		chartName := "stable/wordpress"
		By("Install chart stable/wordpress")
		releaseName, err := helm.Install(chartName, nil)
		Expect(err).NotTo(HaveOccurred())
		By("Check status of release " + releaseName)
		Expect(helm.Status(releaseName)).NotTo(HaveOccurred())
		By("Upgrading release " + releaseName)
		Expect(helm.Upgrade(chartName, releaseName, map[string]string{"image": "bitnami/wordpress:4.7.3-r1"})).NotTo(HaveOccurred())
		By("Rolling back release " + releaseName + "to a first revision")
		Expect(helm.Rollback(releaseName, 1)).NotTo(HaveOccurred())
		By("Deleting release " + releaseName)
		Expect(helm.Delete(releaseName)).NotTo(HaveOccurred())
	})
})
