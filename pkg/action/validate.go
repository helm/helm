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
	"fmt"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/cli-runtime/pkg/resource"

	"helm.sh/helm/v3/pkg/kube"
)

func existingResourceConflict(resources kube.ResourceList) error {
	err := resources.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		helper := resource.NewHelper(info.Client, info.Mapping)
		if _, err := helper.Get(info.Namespace, info.Name, info.Export); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}

			return errors.Wrap(err, "could not get information about the resource")
		}

		return fmt.Errorf("existing resource conflict: kind: %s, namespace: %s, name: %s", info.Mapping.GroupVersionKind.Kind, info.Namespace, info.Name)
	})
	return err
}
