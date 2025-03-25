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

package kube // import "helm.sh/helm/v4/pkg/kube"

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

const defaultNamespace = metav1.NamespaceDefault

func Test_ReadyChecker_IsReady_Pod(t *testing.T) {
	type fields struct {
		client        kubernetes.Interface
		log           Logger
		checkJobs     bool
		pausedAsReady bool
	}
	type args struct {
		ctx      context.Context
		resource *resource.Info
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		pod     *corev1.Pod
		want    bool
		wantErr bool
	}{
		{
			name: "IsReady Pod",
			fields: fields{
				client:        fake.NewSimpleClientset(),
				log:           DefaultLogger,
				checkJobs:     true,
				pausedAsReady: false,
			},
			args: args{
				ctx:      context.TODO(),
				resource: &resource.Info{Object: &corev1.Pod{}, Name: "foo", Namespace: defaultNamespace},
			},
			pod:     newPodWithCondition("foo", corev1.ConditionTrue),
			want:    true,
			wantErr: false,
		},
		{
			name: "IsReady Pod returns error",
			fields: fields{
				client:        fake.NewSimpleClientset(),
				log:           DefaultLogger,
				checkJobs:     true,
				pausedAsReady: false,
			},
			args: args{
				ctx:      context.TODO(),
				resource: &resource.Info{Object: &corev1.Pod{}, Name: "foo", Namespace: defaultNamespace},
			},
			pod:     newPodWithCondition("bar", corev1.ConditionTrue),
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ReadyChecker{
				client:        tt.fields.client,
				log:           tt.fields.log,
				checkJobs:     tt.fields.checkJobs,
				pausedAsReady: tt.fields.pausedAsReady,
			}
			if _, err := c.client.CoreV1().Pods(defaultNamespace).Create(context.TODO(), tt.pod, metav1.CreateOptions{}); err != nil {
				t.Errorf("Failed to create Pod error: %v", err)
				return
			}
			got, err := c.IsReady(tt.args.ctx, tt.args.resource)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsReady() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ReadyChecker_IsReady_Job(t *testing.T) {
	type fields struct {
		client        kubernetes.Interface
		log           Logger
		checkJobs     bool
		pausedAsReady bool
	}
	type args struct {
		ctx      context.Context
		resource *resource.Info
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		job     *batchv1.Job
		want    bool
		wantErr bool
	}{
		{
			name: "IsReady Job error while getting job",
			fields: fields{
				client:        fake.NewSimpleClientset(),
				log:           DefaultLogger,
				checkJobs:     true,
				pausedAsReady: false,
			},
			args: args{
				ctx:      context.TODO(),
				resource: &resource.Info{Object: &batchv1.Job{}, Name: "foo", Namespace: defaultNamespace},
			},
			job:     newJob("bar", 1, intToInt32(1), 1, 0),
			want:    false,
			wantErr: true,
		},
		{
			name: "IsReady Job",
			fields: fields{
				client:        fake.NewSimpleClientset(),
				log:           DefaultLogger,
				checkJobs:     true,
				pausedAsReady: false,
			},
			args: args{
				ctx:      context.TODO(),
				resource: &resource.Info{Object: &batchv1.Job{}, Name: "foo", Namespace: defaultNamespace},
			},
			job:     newJob("foo", 1, intToInt32(1), 1, 0),
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ReadyChecker{
				client:        tt.fields.client,
				log:           tt.fields.log,
				checkJobs:     tt.fields.checkJobs,
				pausedAsReady: tt.fields.pausedAsReady,
			}
			if _, err := c.client.BatchV1().Jobs(defaultNamespace).Create(context.TODO(), tt.job, metav1.CreateOptions{}); err != nil {
				t.Errorf("Failed to create Job error: %v", err)
				return
			}
			got, err := c.IsReady(tt.args.ctx, tt.args.resource)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsReady() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("IsReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ReadyChecker_IsReady_Deployment(t *testing.T) {
	type fields struct {
		client        kubernetes.Interface
		log           Logger
		checkJobs     bool
		pausedAsReady bool
	}
	type args struct {
		ctx      context.Context
		resource *resource.Info
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		replicaSet *appsv1.ReplicaSet
		deployment *appsv1.Deployment
		want       bool
		wantErr    bool
	}{
		{
			name: "IsReady Deployments error while getting current Deployment",
			fields: fields{
				client:        fake.NewSimpleClientset(),
				log:           DefaultLogger,
				checkJobs:     true,
				pausedAsReady: false,
			},
			args: args{
				ctx:      context.TODO(),
				resource: &resource.Info{Object: &appsv1.Deployment{}, Name: "foo", Namespace: defaultNamespace},
			},
			replicaSet: newReplicaSet("foo", 0, 0, true),
			deployment: newDeployment("bar", 1, 1, 0, true),
			want:       false,
			wantErr:    true,
		},
		{
			name: "IsReady Deployments", //TODO fix this one
			fields: fields{
				client:        fake.NewSimpleClientset(),
				log:           DefaultLogger,
				checkJobs:     true,
				pausedAsReady: false,
			},
			args: args{
				ctx:      context.TODO(),
				resource: &resource.Info{Object: &appsv1.Deployment{}, Name: "foo", Namespace: defaultNamespace},
			},
			replicaSet: newReplicaSet("foo", 0, 0, true),
			deployment: newDeployment("foo", 1, 1, 0, true),
			want:       false,
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ReadyChecker{
				client:        tt.fields.client,
				log:           tt.fields.log,
				checkJobs:     tt.fields.checkJobs,
				pausedAsReady: tt.fields.pausedAsReady,
			}
			if _, err := c.client.AppsV1().Deployments(defaultNamespace).Create(context.TODO(), tt.deployment, metav1.CreateOptions{}); err != nil {
				t.Errorf("Failed to create Deployment error: %v", err)
				return
			}
			if _, err := c.client.AppsV1().ReplicaSets(defaultNamespace).Create(context.TODO(), tt.replicaSet, metav1.CreateOptions{}); err != nil {
				t.Errorf("Failed to create ReplicaSet error: %v", err)
				return
			}
			got, err := c.IsReady(tt.args.ctx, tt.args.resource)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsReady() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("IsReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ReadyChecker_IsReady_PersistentVolumeClaim(t *testing.T) {
	type fields struct {
		client        kubernetes.Interface
		log           Logger
		checkJobs     bool
		pausedAsReady bool
	}
	type args struct {
		ctx      context.Context
		resource *resource.Info
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		pvc     *corev1.PersistentVolumeClaim
		want    bool
		wantErr bool
	}{
		{
			name: "IsReady PersistentVolumeClaim",
			fields: fields{
				client:        fake.NewSimpleClientset(),
				log:           DefaultLogger,
				checkJobs:     true,
				pausedAsReady: false,
			},
			args: args{
				ctx:      context.TODO(),
				resource: &resource.Info{Object: &corev1.PersistentVolumeClaim{}, Name: "foo", Namespace: defaultNamespace},
			},
			pvc:     newPersistentVolumeClaim("foo", corev1.ClaimPending),
			want:    false,
			wantErr: false,
		},
		{
			name: "IsReady PersistentVolumeClaim with error",
			fields: fields{
				client:        fake.NewSimpleClientset(),
				log:           DefaultLogger,
				checkJobs:     true,
				pausedAsReady: false,
			},
			args: args{
				ctx:      context.TODO(),
				resource: &resource.Info{Object: &corev1.PersistentVolumeClaim{}, Name: "foo", Namespace: defaultNamespace},
			},
			pvc:     newPersistentVolumeClaim("bar", corev1.ClaimPending),
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ReadyChecker{
				client:        tt.fields.client,
				log:           tt.fields.log,
				checkJobs:     tt.fields.checkJobs,
				pausedAsReady: tt.fields.pausedAsReady,
			}
			if _, err := c.client.CoreV1().PersistentVolumeClaims(defaultNamespace).Create(context.TODO(), tt.pvc, metav1.CreateOptions{}); err != nil {
				t.Errorf("Failed to create PersistentVolumeClaim error: %v", err)
				return
			}
			got, err := c.IsReady(tt.args.ctx, tt.args.resource)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsReady() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("IsReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ReadyChecker_IsReady_Service(t *testing.T) {
	type fields struct {
		client        kubernetes.Interface
		log           Logger
		checkJobs     bool
		pausedAsReady bool
	}
	type args struct {
		ctx      context.Context
		resource *resource.Info
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		svc     *corev1.Service
		want    bool
		wantErr bool
	}{
		{
			name: "IsReady Service",
			fields: fields{
				client:        fake.NewSimpleClientset(),
				log:           DefaultLogger,
				checkJobs:     true,
				pausedAsReady: false,
			},
			args: args{
				ctx:      context.TODO(),
				resource: &resource.Info{Object: &corev1.Service{}, Name: "foo", Namespace: defaultNamespace},
			},
			svc:     newService("foo", corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer, ClusterIP: ""}),
			want:    false,
			wantErr: false,
		},
		{
			name: "IsReady Service with error",
			fields: fields{
				client:        fake.NewSimpleClientset(),
				log:           DefaultLogger,
				checkJobs:     true,
				pausedAsReady: false,
			},
			args: args{
				ctx:      context.TODO(),
				resource: &resource.Info{Object: &corev1.Service{}, Name: "foo", Namespace: defaultNamespace},
			},
			svc:     newService("bar", corev1.ServiceSpec{Type: corev1.ServiceTypeExternalName, ClusterIP: ""}),
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ReadyChecker{
				client:        tt.fields.client,
				log:           tt.fields.log,
				checkJobs:     tt.fields.checkJobs,
				pausedAsReady: tt.fields.pausedAsReady,
			}
			if _, err := c.client.CoreV1().Services(defaultNamespace).Create(context.TODO(), tt.svc, metav1.CreateOptions{}); err != nil {
				t.Errorf("Failed to create Service error: %v", err)
				return
			}
			got, err := c.IsReady(tt.args.ctx, tt.args.resource)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsReady() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("IsReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ReadyChecker_IsReady_DaemonSet(t *testing.T) {
	type fields struct {
		client        kubernetes.Interface
		log           Logger
		checkJobs     bool
		pausedAsReady bool
	}
	type args struct {
		ctx      context.Context
		resource *resource.Info
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		ds      *appsv1.DaemonSet
		want    bool
		wantErr bool
	}{
		{
			name: "IsReady DaemonSet",
			fields: fields{
				client:        fake.NewSimpleClientset(),
				log:           DefaultLogger,
				checkJobs:     true,
				pausedAsReady: false,
			},
			args: args{
				ctx:      context.TODO(),
				resource: &resource.Info{Object: &appsv1.DaemonSet{}, Name: "foo", Namespace: defaultNamespace},
			},
			ds:      newDaemonSet("foo", 0, 0, 1, 0, true),
			want:    false,
			wantErr: false,
		},
		{
			name: "IsReady DaemonSet with error",
			fields: fields{
				client:        fake.NewSimpleClientset(),
				log:           DefaultLogger,
				checkJobs:     true,
				pausedAsReady: false,
			},
			args: args{
				ctx:      context.TODO(),
				resource: &resource.Info{Object: &appsv1.DaemonSet{}, Name: "foo", Namespace: defaultNamespace},
			},
			ds:      newDaemonSet("bar", 0, 1, 1, 1, true),
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ReadyChecker{
				client:        tt.fields.client,
				log:           tt.fields.log,
				checkJobs:     tt.fields.checkJobs,
				pausedAsReady: tt.fields.pausedAsReady,
			}
			if _, err := c.client.AppsV1().DaemonSets(defaultNamespace).Create(context.TODO(), tt.ds, metav1.CreateOptions{}); err != nil {
				t.Errorf("Failed to create DaemonSet error: %v", err)
				return
			}
			got, err := c.IsReady(tt.args.ctx, tt.args.resource)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsReady() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("IsReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ReadyChecker_IsReady_StatefulSet(t *testing.T) {
	type fields struct {
		client        kubernetes.Interface
		log           Logger
		checkJobs     bool
		pausedAsReady bool
	}
	type args struct {
		ctx      context.Context
		resource *resource.Info
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		ss      *appsv1.StatefulSet
		want    bool
		wantErr bool
	}{
		{
			name: "IsReady StatefulSet",
			fields: fields{
				client:        fake.NewSimpleClientset(),
				log:           DefaultLogger,
				checkJobs:     true,
				pausedAsReady: false,
			},
			args: args{
				ctx:      context.TODO(),
				resource: &resource.Info{Object: &appsv1.StatefulSet{}, Name: "foo", Namespace: defaultNamespace},
			},
			ss:      newStatefulSet("foo", 1, 0, 0, 1, true),
			want:    false,
			wantErr: false,
		},
		{
			name: "IsReady StatefulSet with error",
			fields: fields{
				client:        fake.NewSimpleClientset(),
				log:           DefaultLogger,
				checkJobs:     true,
				pausedAsReady: false,
			},
			args: args{
				ctx:      context.TODO(),
				resource: &resource.Info{Object: &appsv1.StatefulSet{}, Name: "foo", Namespace: defaultNamespace},
			},
			ss:      newStatefulSet("bar", 1, 0, 1, 1, true),
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ReadyChecker{
				client:        tt.fields.client,
				log:           tt.fields.log,
				checkJobs:     tt.fields.checkJobs,
				pausedAsReady: tt.fields.pausedAsReady,
			}
			if _, err := c.client.AppsV1().StatefulSets(defaultNamespace).Create(context.TODO(), tt.ss, metav1.CreateOptions{}); err != nil {
				t.Errorf("Failed to create StatefulSet error: %v", err)
				return
			}
			got, err := c.IsReady(tt.args.ctx, tt.args.resource)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsReady() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("IsReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ReadyChecker_IsReady_ReplicationController(t *testing.T) {
	type fields struct {
		client        kubernetes.Interface
		log           Logger
		checkJobs     bool
		pausedAsReady bool
	}
	type args struct {
		ctx      context.Context
		resource *resource.Info
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		rc      *corev1.ReplicationController
		want    bool
		wantErr bool
	}{
		{
			name: "IsReady ReplicationController",
			fields: fields{
				client:        fake.NewSimpleClientset(),
				log:           DefaultLogger,
				checkJobs:     true,
				pausedAsReady: false,
			},
			args: args{
				ctx:      context.TODO(),
				resource: &resource.Info{Object: &corev1.ReplicationController{}, Name: "foo", Namespace: defaultNamespace},
			},
			rc:      newReplicationController("foo", false),
			want:    false,
			wantErr: false,
		},
		{
			name: "IsReady ReplicationController with error",
			fields: fields{
				client:        fake.NewSimpleClientset(),
				log:           DefaultLogger,
				checkJobs:     true,
				pausedAsReady: false,
			},
			args: args{
				ctx:      context.TODO(),
				resource: &resource.Info{Object: &corev1.ReplicationController{}, Name: "foo", Namespace: defaultNamespace},
			},
			rc:      newReplicationController("bar", false),
			want:    false,
			wantErr: true,
		},
		{
			name: "IsReady ReplicationController and pods not ready for object",
			fields: fields{
				client:        fake.NewSimpleClientset(),
				log:           DefaultLogger,
				checkJobs:     true,
				pausedAsReady: false,
			},
			args: args{
				ctx:      context.TODO(),
				resource: &resource.Info{Object: &corev1.ReplicationController{}, Name: "foo", Namespace: defaultNamespace},
			},
			rc:      newReplicationController("foo", true),
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ReadyChecker{
				client:        tt.fields.client,
				log:           tt.fields.log,
				checkJobs:     tt.fields.checkJobs,
				pausedAsReady: tt.fields.pausedAsReady,
			}
			if _, err := c.client.CoreV1().ReplicationControllers(defaultNamespace).Create(context.TODO(), tt.rc, metav1.CreateOptions{}); err != nil {
				t.Errorf("Failed to create ReplicationController error: %v", err)
				return
			}
			got, err := c.IsReady(tt.args.ctx, tt.args.resource)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsReady() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("IsReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ReadyChecker_IsReady_ReplicaSet(t *testing.T) {
	type fields struct {
		client        kubernetes.Interface
		log           Logger
		checkJobs     bool
		pausedAsReady bool
	}
	type args struct {
		ctx      context.Context
		resource *resource.Info
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		rs      *appsv1.ReplicaSet
		want    bool
		wantErr bool
	}{
		{
			name: "IsReady ReplicaSet",
			fields: fields{
				client:        fake.NewSimpleClientset(),
				log:           DefaultLogger,
				checkJobs:     true,
				pausedAsReady: false,
			},
			args: args{
				ctx:      context.TODO(),
				resource: &resource.Info{Object: &appsv1.ReplicaSet{}, Name: "foo", Namespace: defaultNamespace},
			},
			rs:      newReplicaSet("foo", 1, 1, true),
			want:    false,
			wantErr: true,
		},
		{
			name: "IsReady ReplicaSet not ready",
			fields: fields{
				client:        fake.NewSimpleClientset(),
				log:           DefaultLogger,
				checkJobs:     true,
				pausedAsReady: false,
			},
			args: args{
				ctx:      context.TODO(),
				resource: &resource.Info{Object: &appsv1.ReplicaSet{}, Name: "foo", Namespace: defaultNamespace},
			},
			rs:      newReplicaSet("bar", 1, 1, false),
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ReadyChecker{
				client:        tt.fields.client,
				log:           tt.fields.log,
				checkJobs:     tt.fields.checkJobs,
				pausedAsReady: tt.fields.pausedAsReady,
			}
			//
			got, err := c.IsReady(tt.args.ctx, tt.args.resource)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsReady() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("IsReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ReadyChecker_deploymentReady(t *testing.T) {
	type args struct {
		rs  *appsv1.ReplicaSet
		dep *appsv1.Deployment
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "deployment is ready",
			args: args{
				rs:  newReplicaSet("foo", 1, 1, true),
				dep: newDeployment("foo", 1, 1, 0, true),
			},
			want: true,
		},
		{
			name: "deployment is not ready",
			args: args{
				rs:  newReplicaSet("foo", 0, 0, true),
				dep: newDeployment("foo", 1, 1, 0, true),
			},
			want: false,
		},
		{
			name: "deployment is ready when maxUnavailable is set",
			args: args{
				rs:  newReplicaSet("foo", 2, 1, true),
				dep: newDeployment("foo", 2, 1, 1, true),
			},
			want: true,
		},
		{
			name: "deployment is not ready when replicaset generations are out of sync",
			args: args{
				rs:  newReplicaSet("foo", 1, 1, false),
				dep: newDeployment("foo", 1, 1, 0, true),
			},
			want: false,
		},
		{
			name: "deployment is not ready when deployment generations are out of sync",
			args: args{
				rs:  newReplicaSet("foo", 1, 1, true),
				dep: newDeployment("foo", 1, 1, 0, false),
			},
			want: false,
		},
		{
			name: "deployment is not ready when generations are out of sync",
			args: args{
				rs:  newReplicaSet("foo", 1, 1, false),
				dep: newDeployment("foo", 1, 1, 0, false),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewReadyChecker(fake.NewSimpleClientset(), nil)
			if got := c.deploymentReady(tt.args.rs, tt.args.dep); got != tt.want {
				t.Errorf("deploymentReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ReadyChecker_replicaSetReady(t *testing.T) {
	type args struct {
		rs *appsv1.ReplicaSet
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "replicaSet is ready",
			args: args{
				rs: newReplicaSet("foo", 1, 1, true),
			},
			want: true,
		},
		{
			name: "replicaSet is not ready when generations are out of sync",
			args: args{
				rs: newReplicaSet("foo", 1, 1, false),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewReadyChecker(fake.NewSimpleClientset(), nil)
			if got := c.replicaSetReady(tt.args.rs); got != tt.want {
				t.Errorf("replicaSetReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ReadyChecker_replicationControllerReady(t *testing.T) {
	type args struct {
		rc *corev1.ReplicationController
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "replicationController is ready",
			args: args{
				rc: newReplicationController("foo", true),
			},
			want: true,
		},
		{
			name: "replicationController is not ready when generations are out of sync",
			args: args{
				rc: newReplicationController("foo", false),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewReadyChecker(fake.NewSimpleClientset(), nil)
			if got := c.replicationControllerReady(tt.args.rc); got != tt.want {
				t.Errorf("replicationControllerReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ReadyChecker_daemonSetReady(t *testing.T) {
	type args struct {
		ds *appsv1.DaemonSet
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "daemonset is ready",
			args: args{
				ds: newDaemonSet("foo", 0, 1, 1, 1, true),
			},
			want: true,
		},
		{
			name: "daemonset is not ready",
			args: args{
				ds: newDaemonSet("foo", 0, 0, 1, 1, true),
			},
			want: false,
		},
		{
			name: "daemonset pods have not been scheduled successfully",
			args: args{
				ds: newDaemonSet("foo", 0, 0, 1, 0, true),
			},
			want: false,
		},
		{
			name: "daemonset is ready when maxUnavailable is set",
			args: args{
				ds: newDaemonSet("foo", 1, 1, 2, 2, true),
			},
			want: true,
		},
		{
			name: "daemonset is not ready when generations are out of sync",
			args: args{
				ds: newDaemonSet("foo", 0, 1, 1, 1, false),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewReadyChecker(fake.NewSimpleClientset(), nil)
			if got := c.daemonSetReady(tt.args.ds); got != tt.want {
				t.Errorf("daemonSetReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ReadyChecker_statefulSetReady(t *testing.T) {
	type args struct {
		sts *appsv1.StatefulSet
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "statefulset is ready",
			args: args{
				sts: newStatefulSet("foo", 1, 0, 1, 1, true),
			},
			want: true,
		},
		{
			name: "statefulset is not ready",
			args: args{
				sts: newStatefulSet("foo", 1, 0, 0, 1, true),
			},
			want: false,
		},
		{
			name: "statefulset is ready when partition is specified",
			args: args{
				sts: newStatefulSet("foo", 2, 1, 2, 1, true),
			},
			want: true,
		},
		{
			name: "statefulset is not ready when partition is set",
			args: args{
				sts: newStatefulSet("foo", 2, 1, 1, 0, true),
			},
			want: false,
		},
		{
			name: "statefulset is ready when partition is set and no change in template",
			args: args{
				sts: newStatefulSet("foo", 2, 1, 2, 2, true),
			},
			want: true,
		},
		{
			name: "statefulset is ready when partition is greater than replicas",
			args: args{
				sts: newStatefulSet("foo", 1, 2, 1, 1, true),
			},
			want: true,
		},
		{
			name: "statefulset is not ready when generations are out of sync",
			args: args{
				sts: newStatefulSet("foo", 1, 0, 1, 1, false),
			},
			want: false,
		},
		{
			name: "statefulset is ready when current revision for current replicas does not match update revision for updated replicas when using partition !=0",
			args: args{
				sts: newStatefulSetWithUpdateRevision("foo", 3, 2, 3, 3, "foo-bbbbbbb", true),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewReadyChecker(fake.NewSimpleClientset(), nil)
			if got := c.statefulSetReady(tt.args.sts); got != tt.want {
				t.Errorf("statefulSetReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ReadyChecker_podsReadyForObject(t *testing.T) {
	type args struct {
		namespace string
		obj       runtime.Object
	}
	tests := []struct {
		name      string
		args      args
		existPods []corev1.Pod
		want      bool
		wantErr   bool
	}{
		{
			name: "pods ready for a replicaset",
			args: args{
				namespace: defaultNamespace,
				obj:       newReplicaSet("foo", 1, 1, true),
			},
			existPods: []corev1.Pod{
				*newPodWithCondition("foo", corev1.ConditionTrue),
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "pods not ready for a replicaset",
			args: args{
				namespace: defaultNamespace,
				obj:       newReplicaSet("foo", 1, 1, true),
			},
			existPods: []corev1.Pod{
				*newPodWithCondition("foo", corev1.ConditionFalse),
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "ReplicaSet not set",
			args: args{
				namespace: defaultNamespace,
				obj:       nil,
			},
			existPods: []corev1.Pod{
				*newPodWithCondition("foo", corev1.ConditionFalse),
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewReadyChecker(fake.NewSimpleClientset(), nil)
			for _, pod := range tt.existPods {
				if _, err := c.client.CoreV1().Pods(defaultNamespace).Create(context.TODO(), &pod, metav1.CreateOptions{}); err != nil {
					t.Errorf("Failed to create Pod error: %v", err)
					return
				}
			}
			got, err := c.podsReadyForObject(context.TODO(), tt.args.namespace, tt.args.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("podsReadyForObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("podsReadyForObject() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ReadyChecker_jobReady(t *testing.T) {
	type args struct {
		job *batchv1.Job
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name:    "job is completed",
			args:    args{job: newJob("foo", 1, intToInt32(1), 1, 0)},
			want:    true,
			wantErr: false,
		},
		{
			name:    "job is incomplete",
			args:    args{job: newJob("foo", 1, intToInt32(1), 0, 0)},
			want:    false,
			wantErr: false,
		},
		{
			name:    "job is failed but within BackoffLimit",
			args:    args{job: newJob("foo", 1, intToInt32(1), 0, 1)},
			want:    false,
			wantErr: false,
		},
		{
			name:    "job is completed with retry",
			args:    args{job: newJob("foo", 1, intToInt32(1), 1, 1)},
			want:    true,
			wantErr: false,
		},
		{
			name:    "job is failed and beyond BackoffLimit",
			args:    args{job: newJob("foo", 1, intToInt32(1), 0, 2)},
			want:    false,
			wantErr: true,
		},
		{
			name:    "job is completed single run",
			args:    args{job: newJob("foo", 0, intToInt32(1), 1, 0)},
			want:    true,
			wantErr: false,
		},
		{
			name:    "job is failed single run",
			args:    args{job: newJob("foo", 0, intToInt32(1), 0, 1)},
			want:    false,
			wantErr: true,
		},
		{
			name: "job with null completions",
			args: args{job: newJob("foo", 0, nil, 1, 0)},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewReadyChecker(fake.NewSimpleClientset(), nil)
			got, err := c.jobReady(tt.args.job)
			if (err != nil) != tt.wantErr {
				t.Errorf("jobReady() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("jobReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ReadyChecker_volumeReady(t *testing.T) {
	type args struct {
		v *corev1.PersistentVolumeClaim
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "pvc is bound",
			args: args{
				v: newPersistentVolumeClaim("foo", corev1.ClaimBound),
			},
			want: true,
		},
		{
			name: "pvc is not ready",
			args: args{
				v: newPersistentVolumeClaim("foo", corev1.ClaimPending),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewReadyChecker(fake.NewSimpleClientset(), nil)
			if got := c.volumeReady(tt.args.v); got != tt.want {
				t.Errorf("volumeReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ReadyChecker_serviceReady(t *testing.T) {
	type args struct {
		service *corev1.Service
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "service type is of external name",
			args: args{service: newService("foo", corev1.ServiceSpec{Type: corev1.ServiceTypeExternalName, ClusterIP: ""})},
			want: true,
		},
		{
			name: "service cluster ip is empty",
			args: args{service: newService("foo", corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer, ClusterIP: ""})},
			want: false,
		},
		{
			name: "service has a cluster ip that is greater than 0",
			args: args{service: newService("foo", corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer, ClusterIP: "bar", ExternalIPs: []string{"bar"}})},
			want: true,
		},
		{
			name: "service has a cluster ip that is less than 0 and ingress is nil",
			args: args{service: newService("foo", corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer, ClusterIP: "bar"})},
			want: false,
		},
		{
			name: "service has a cluster ip that is less than 0 and ingress is nil",
			args: args{service: newService("foo", corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, ClusterIP: "bar"})},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewReadyChecker(fake.NewSimpleClientset(), nil)
			got := c.serviceReady(tt.args.service)
			if got != tt.want {
				t.Errorf("serviceReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ReadyChecker_crdBetaReady(t *testing.T) {
	type args struct {
		crdBeta apiextv1beta1.CustomResourceDefinition
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "crdBeta type is Establish and Conditional is true",
			args: args{crdBeta: newcrdBetaReady("foo", apiextv1beta1.CustomResourceDefinitionStatus{
				Conditions: []apiextv1beta1.CustomResourceDefinitionCondition{
					{
						Type:   apiextv1beta1.Established,
						Status: apiextv1beta1.ConditionTrue,
					},
				},
			})},
			want: true,
		},
		{
			name: "crdBeta type is Establish and Conditional is false",
			args: args{crdBeta: newcrdBetaReady("foo", apiextv1beta1.CustomResourceDefinitionStatus{
				Conditions: []apiextv1beta1.CustomResourceDefinitionCondition{
					{
						Type:   apiextv1beta1.Established,
						Status: apiextv1beta1.ConditionFalse,
					},
				},
			})},
			want: false,
		},
		{
			name: "crdBeta type is NamesAccepted and Conditional is true",
			args: args{crdBeta: newcrdBetaReady("foo", apiextv1beta1.CustomResourceDefinitionStatus{
				Conditions: []apiextv1beta1.CustomResourceDefinitionCondition{
					{
						Type:   apiextv1beta1.NamesAccepted,
						Status: apiextv1beta1.ConditionTrue,
					},
				},
			})},
			want: false,
		},
		{
			name: "crdBeta type is NamesAccepted and Conditional is false",
			args: args{crdBeta: newcrdBetaReady("foo", apiextv1beta1.CustomResourceDefinitionStatus{
				Conditions: []apiextv1beta1.CustomResourceDefinitionCondition{
					{
						Type:   apiextv1beta1.NamesAccepted,
						Status: apiextv1beta1.ConditionFalse,
					},
				},
			})},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewReadyChecker(fake.NewSimpleClientset(), nil)
			got := c.crdBetaReady(tt.args.crdBeta)
			if got != tt.want {
				t.Errorf("crdBetaReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ReadyChecker_crdReady(t *testing.T) {
	type args struct {
		crdBeta apiextv1.CustomResourceDefinition
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "crdBeta type is Establish and Conditional is true",
			args: args{crdBeta: newcrdReady("foo", apiextv1.CustomResourceDefinitionStatus{
				Conditions: []apiextv1.CustomResourceDefinitionCondition{
					{
						Type:   apiextv1.Established,
						Status: apiextv1.ConditionTrue,
					},
				},
			})},
			want: true,
		},
		{
			name: "crdBeta type is Establish and Conditional is false",
			args: args{crdBeta: newcrdReady("foo", apiextv1.CustomResourceDefinitionStatus{
				Conditions: []apiextv1.CustomResourceDefinitionCondition{
					{
						Type:   apiextv1.Established,
						Status: apiextv1.ConditionFalse,
					},
				},
			})},
			want: false,
		},
		{
			name: "crdBeta type is NamesAccepted and Conditional is true",
			args: args{crdBeta: newcrdReady("foo", apiextv1.CustomResourceDefinitionStatus{
				Conditions: []apiextv1.CustomResourceDefinitionCondition{
					{
						Type:   apiextv1.NamesAccepted,
						Status: apiextv1.ConditionTrue,
					},
				},
			})},
			want: false,
		},
		{
			name: "crdBeta type is NamesAccepted and Conditional is false",
			args: args{crdBeta: newcrdReady("foo", apiextv1.CustomResourceDefinitionStatus{
				Conditions: []apiextv1.CustomResourceDefinitionCondition{
					{
						Type:   apiextv1.NamesAccepted,
						Status: apiextv1.ConditionFalse,
					},
				},
			})},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewReadyChecker(fake.NewSimpleClientset(), nil)
			got := c.crdReady(tt.args.crdBeta)
			if got != tt.want {
				t.Errorf("crdBetaReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func newStatefulSetWithUpdateRevision(name string, replicas, partition, readyReplicas, updatedReplicas int, updateRevision string, generationInSync bool) *appsv1.StatefulSet {
	ss := newStatefulSet(name, replicas, partition, readyReplicas, updatedReplicas, generationInSync)
	ss.Status.UpdateRevision = updateRevision
	return ss
}

func newDaemonSet(name string, maxUnavailable, numberReady, desiredNumberScheduled, updatedNumberScheduled int, generationInSync bool) *appsv1.DaemonSet {
	var generation, observedGeneration int64 = 1, 1
	if !generationInSync {
		generation = 2
	}
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  defaultNamespace,
			Generation: generation,
		},
		Spec: appsv1.DaemonSetSpec{
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: func() *intstr.IntOrString { i := intstr.FromInt(maxUnavailable); return &i }(),
				},
			},
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"name": name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   name,
					Labels: map[string]string{"name": name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: "nginx",
						},
					},
				},
			},
		},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: int32(desiredNumberScheduled),
			NumberReady:            int32(numberReady),
			UpdatedNumberScheduled: int32(updatedNumberScheduled),
			ObservedGeneration:     observedGeneration,
		},
	}
}

func newStatefulSet(name string, replicas, partition, readyReplicas, updatedReplicas int, generationInSync bool) *appsv1.StatefulSet {
	var generation, observedGeneration int64 = 1, 1
	if !generationInSync {
		generation = 2
	}
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  defaultNamespace,
			Generation: generation,
		},
		Spec: appsv1.StatefulSetSpec{
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
					Partition: intToInt32(partition),
				},
			},
			Replicas: intToInt32(replicas),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"name": name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   name,
					Labels: map[string]string{"name": name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: "nginx",
						},
					},
				},
			},
		},
		Status: appsv1.StatefulSetStatus{
			UpdatedReplicas:    int32(updatedReplicas),
			ReadyReplicas:      int32(readyReplicas),
			ObservedGeneration: observedGeneration,
		},
	}
}

func newDeployment(name string, replicas, maxSurge, maxUnavailable int, generationInSync bool) *appsv1.Deployment {
	var generation, observedGeneration int64 = 1, 1
	if !generationInSync {
		generation = 2
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  defaultNamespace,
			Generation: generation,
		},
		Spec: appsv1.DeploymentSpec{
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: func() *intstr.IntOrString { i := intstr.FromInt(maxUnavailable); return &i }(),
					MaxSurge:       func() *intstr.IntOrString { i := intstr.FromInt(maxSurge); return &i }(),
				},
			},
			Replicas: intToInt32(replicas),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"name": name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   name,
					Labels: map[string]string{"name": name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: "nginx",
						},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: observedGeneration,
		},
	}
}

func newReplicationController(name string, generationInSync bool) *corev1.ReplicationController {
	var generation, observedGeneration int64 = 1, 1
	if !generationInSync {
		generation = 2
	}
	return &corev1.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Generation: generation,
		},
		Status: corev1.ReplicationControllerStatus{
			ObservedGeneration: observedGeneration,
		},
	}
}

func newReplicaSet(name string, replicas int, readyReplicas int, generationInSync bool) *appsv1.ReplicaSet {
	d := newDeployment(name, replicas, 0, 0, generationInSync)
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       defaultNamespace,
			Labels:          d.Spec.Selector.MatchLabels,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(d, d.GroupVersionKind())},
			Generation:      d.Generation,
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: d.Spec.Selector,
			Replicas: intToInt32(replicas),
			Template: d.Spec.Template,
		},
		Status: appsv1.ReplicaSetStatus{
			ReadyReplicas:      int32(readyReplicas),
			ObservedGeneration: d.Status.ObservedGeneration,
		},
	}
}

func newPodWithCondition(name string, podReadyCondition corev1.ConditionStatus) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: defaultNamespace,
			Labels:    map[string]string{"name": name},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Image: "nginx",
				},
			},
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: podReadyCondition,
				},
			},
		},
	}
}

func newPersistentVolumeClaim(name string, phase corev1.PersistentVolumeClaimPhase) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: defaultNamespace,
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: phase,
		},
	}
}

func newJob(name string, backoffLimit int, completions *int32, succeeded int, failed int) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: defaultNamespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: intToInt32(backoffLimit),
			Completions:  completions,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   name,
					Labels: map[string]string{"name": name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: "nginx",
						},
					},
				},
			},
		},
		Status: batchv1.JobStatus{
			Succeeded: int32(succeeded),
			Failed:    int32(failed),
		},
	}
}

func newService(name string, serviceSpec corev1.ServiceSpec) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: defaultNamespace,
		},
		Spec: serviceSpec,
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: nil,
			},
		},
	}
}

func newcrdBetaReady(name string, crdBetaStatus apiextv1beta1.CustomResourceDefinitionStatus) apiextv1beta1.CustomResourceDefinition {
	return apiextv1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: defaultNamespace,
		},
		Spec:   apiextv1beta1.CustomResourceDefinitionSpec{},
		Status: crdBetaStatus,
	}
}

func newcrdReady(name string, crdBetaStatus apiextv1.CustomResourceDefinitionStatus) apiextv1.CustomResourceDefinition {
	return apiextv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: defaultNamespace,
		},
		Spec:   apiextv1.CustomResourceDefinitionSpec{},
		Status: crdBetaStatus,
	}
}

func intToInt32(i int) *int32 {
	i32 := int32(i)
	return &i32
}
