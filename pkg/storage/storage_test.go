/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
package storage // import "k8s.io/helm/pkg/storage"

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	rspb "k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/storage/driver"

	tspb "github.com/golang/protobuf/ptypes"
)

var storage = Init(driver.NewMemory())

func releaseData() *rspb.Release {
	var manifest = `apiVersion: v1
	kind: ConfigMap
	metadata:
	  name: configmap-storage-test
	data:
	  count: "100"
	  limit: "200"
	  state: "new"
	  token: "abc"
	`

	tm, _ := tspb.TimestampProto(time.Now())
	return &rspb.Release{
		Name: "hungry-hippo",
		Info: &rspb.Info{
			FirstDeployed: tm,
			LastDeployed:  tm,
			Status:        &rspb.Status{Code: rspb.Status_DEPLOYED},
		},
		Version:   2,
		Manifest:  manifest,
		Namespace: "kube-system",
	}
}

func TestStoreRelease(t *testing.T) {
	ckerr := func(err error, msg string) {
		if err != nil {
			t.Fatalf(fmt.Sprintf("Failed to %s: %q", msg, err))
		}
	}

	rls := releaseData()
	ckerr(storage.StoreRelease(rls), "StoreRelease")

	res, err := storage.QueryRelease(rls.Name)
	ckerr(err, "QueryRelease")

	if !reflect.DeepEqual(rls, res) {
		t.Fatalf("Expected %q, got %q", rls, res)
	}
}

func TestQueryRelease(t *testing.T) {
	ckerr := func(err error, msg string) {
		if err != nil {
			t.Fatalf(fmt.Sprintf("Failed to %s: %q", msg, err))
		}
	}

	rls := releaseData()
	ckerr(storage.StoreRelease(rls), "StoreRelease")

	res, err := storage.QueryRelease(rls.Name)
	ckerr(err, "QueryRelease")

	if !reflect.DeepEqual(rls, res) {
		t.Fatalf("Expected %q, got %q", rls, res)
	}
}

func TestDeleteRelease(t *testing.T) {
	ckerr := func(err error, msg string) {
		if err != nil {
			t.Fatalf(fmt.Sprintf("Failed to %s: %q", msg, err))
		}
	}

	rls := releaseData()
	ckerr(storage.StoreRelease(rls), "StoreRelease")

	res, err := storage.DeleteRelease(rls.Name)
	ckerr(err, "DeleteRelease")

	if !reflect.DeepEqual(rls, res) {
		t.Fatalf("Expected %q, got %q", rls, res)
	}
}

func TestUpdateRelease(t *testing.T) {
	ckeql := func(got, want interface{}, msg string) {
		if !reflect.DeepEqual(got, want) {
			t.Fatalf(fmt.Sprintf("%s: got %T, want %T", msg, got, want))
		}
	}

	ckerr := func(err error, msg string) {
		if err != nil {
			t.Fatalf(fmt.Sprintf("Failed to %s: %q", msg, err))
		}
	}

	rls := releaseData()
	ckerr(storage.StoreRelease(rls), "StoreRelease")

	rls.Name = "hungry-hippo"
	rls.Version = 2
	rls.Manifest = "old-manifest"

	err := storage.UpdateRelease(rls)
	ckerr(err, "UpdateRelease")

	res, err := storage.QueryRelease(rls.Name)
	ckerr(err, "QueryRelease")
	ckeql(res, rls, "Expected Release")
	ckeql(res.Name, rls.Name, "Expected Name")
	ckeql(res.Version, rls.Version, "Expected Version")
	ckeql(res.Manifest, rls.Manifest, "Expected Manifest")
}
