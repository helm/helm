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
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"helm.sh/helm/v3/pkg/release"
)

// ReleaseTesting is the action for testing a release.
//
// It provides the implementation of 'helm test'.
type ReleaseTesting struct {
	cfg       *Configuration
	clientSet kubernetes.Interface
	Timeout   time.Duration
	// Used for fetching logs from test pods
	Namespace string
}

// NewReleaseTesting creates a new ReleaseTesting object with the given configuration.
func NewReleaseTesting(cfg *Configuration) *ReleaseTesting {
	clientSet, _ := cfg.KubernetesClientSet()
	return &ReleaseTesting{
		cfg:       cfg,
		clientSet: clientSet,
	}
}

// Run executes 'helm test' against the given release.
func (r *ReleaseTesting) Run(name string) (*release.Release, error) {
	if err := r.cfg.KubeClient.IsReachable(); err != nil {
		return nil, err
	}

	if err := validateReleaseName(name); err != nil {
		return nil, errors.Errorf("releaseTest: Release name is invalid: %s", name)
	}

	// finds the non-deleted release with the given name
	rel, err := r.cfg.Releases.Last(name)
	if err != nil {
		return rel, err
	}

	if err := r.cfg.execHook(rel, release.HookTest, r.Timeout); err != nil {
		r.cfg.Releases.Update(rel)
		return rel, err
	}

	return rel, r.cfg.Releases.Update(rel)
}

// GetPodLogs will write the logs for all test pods in the given release into
// the given writer. These can be immediately output to the user or captured for
// other uses
func (r *ReleaseTesting) GetPodLogs(out io.Writer, rel *release.Release) error {
	for _, h := range rel.Hooks {
		for _, e := range h.Events {
			if e == release.HookTest {
				var podName string
				if h.Kind == "Job" {
					resp, err := r.clientSet.CoreV1().Pods(r.Namespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: fmt.Sprintf("job-name=%s", h.Name),
					})
					if err != nil || len(resp.Items) == 0 {
						return errors.Wrapf(err, "Unable to get the running pod for %s", h.Name)
					}
					//TODO When multiple pods are parallel, it is necessary to aggregate logs
					podName = resp.Items[0].Name
				} else if h.Kind == "Pod" {
					podName = h.Name
				} else {
					log.Printf("warning: The current type does not support log acquisition, only Job or Pod")
					continue
				}
				req := r.clientSet.CoreV1().Pods(r.Namespace).GetLogs(podName, &v1.PodLogOptions{})
				logReader, err := req.Stream(context.Background())
				if err != nil {
					return errors.Wrapf(err, "unable to get pod logs for %s", podName)
				}

				fmt.Fprintf(out, "POD LOGS: %s\n", podName)
				_, err = io.Copy(out, logReader)
				fmt.Fprintln(out)
				if err != nil {
					return errors.Wrapf(err, "unable to write pod logs for %s", podName)
				}
			}
		}
	}
	return nil
}
