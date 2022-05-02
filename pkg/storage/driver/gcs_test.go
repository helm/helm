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

package driver

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"

	"cloud.google.com/go/httpreplay"
	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	rspb "helm.sh/helm/v3/pkg/release"
)

var (
	gcsNow = "1987-16-01 09:10:11"
	gcsRec = flag.Bool("record", false, "record RPCs")

	gcsDriver *GCS
)

func TestMain(m *testing.M) {
	cleanup := newTestFixtureGCS()

	clearObjects()
	retCode := m.Run()
	clearObjects()

	cleanup()
	os.Exit(retCode)
}

func clearObjects() {
	ctx := context.Background()
	objs := gcsDriver.client.
		Bucket(gcsDriver.bucket).
		Objects(ctx, &storage.Query{Prefix: ""})
	for {
		objAttrs, err := objs.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			panic(err)
		}
		obj := gcsDriver.client.
			Bucket(gcsDriver.bucket).
			Object(objAttrs.Name)
		obj.Delete(ctx)
	}
}

func TestGCSName(t *testing.T) {
	if gcsDriver.Name() != GCSDriverName {
		t.Errorf("Expected name to be %s, got %s", GCSDriverName, gcsDriver.Name())
	}
}

func TestGCSGet(t *testing.T) {
	vers := int(1)
	name := "gcs-test-get"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, "default", rspb.StatusDeployed)

	// create stub
	if err := gcsDriver.Create(key, rel); err != nil {
		t.Fatalf("failed to create release with key %s: %v", key, err)
	}

	// test get release
	got, err := gcsDriver.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release: %v", err)
	}
	if !reflect.DeepEqual(rel, got) {
		t.Errorf("Expected release {%v}, got {%v}", rel, got)
	}
}

func TestGCSGetNotExist(t *testing.T) {
	vers := int(1)
	name := "gcs-test-get-not-exists"
	key := testKey(name, vers)

	got, err := gcsDriver.Get(key)
	if err == nil || got != nil {
		t.Fatal("Release must be not found")
	}
}

func TestGCSCreate(t *testing.T) {
	vers := 1
	name := "gcs-test-create"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, "default", rspb.StatusDeployed)

	if err := gcsDriver.Create(key, rel); err != nil {
		t.Fatalf("failed to create release with key %s: %v", key, err)
	}
}

func TestGCSUpdate(t *testing.T) {
	vers := 1
	name := "gcs-test-update"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, "default", rspb.StatusDeployed)

	// create stub
	if err := gcsDriver.Create(key, rel); err != nil {
		t.Fatalf("failed to create release with key %s: %v", key, err)
	}

	// test update release
	if err := gcsDriver.Update(key, rel); err != nil {
		t.Fatalf("failed to update release with key %s: %v", key, err)
	}
}

func TestGCSDelete(t *testing.T) {
	vers := 1
	name := "gcs-test-delete"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, "default", rspb.StatusDeployed)

	// create stub
	if err := gcsDriver.Create(key, rel); err != nil {
		t.Fatalf("failed to create release with key %s: %v", key, err)
	}

	// test delete release
	deletedRelease, err := gcsDriver.Delete(key)
	if err != nil {
		t.Fatalf("failed to delete release with key %s: %v", key, err)
	}

	if !reflect.DeepEqual(rel, deletedRelease) {
		t.Errorf("Expected release {%v}, got {%v}", rel, deletedRelease)
	}
}

func TestGCSDeleteNotFound(t *testing.T) {
	vers := 1
	name := "gcs-test-delete-not-found"
	key := testKey(name, vers)

	if _, err := gcsDriver.Delete(key); err == nil {
		t.Fatalf("release found with key %s: %v", key, err)
	}
}

func TestGCSList(t *testing.T) {
	gcsDriver.pathPrefix = "helm-tests-list"
	gcsDriver.namespace = ""

	namespaceA := "list-a"
	namespaceB := "list-b"

	tests := []struct {
		key       string
		namespace string
		status    rspb.Status
	}{
		{"gcs-test-list-key-1", namespaceA, rspb.StatusUninstalled},
		{"gcs-test-list-key-2", namespaceA, rspb.StatusUninstalled},
		{"gcs-test-list-key-3", namespaceA, rspb.StatusDeployed},
		{"gcs-test-list-key-4", namespaceB, rspb.StatusDeployed},
		{"gcs-test-list-key-5", namespaceB, rspb.StatusSuperseded},
		{"gcs-test-list-key-6", namespaceB, rspb.StatusSuperseded},
	}

	// create stubs
	for _, tt := range tests {
		rel := releaseStub(tt.key, 1, tt.namespace, tt.status)
		if err := gcsDriver.Create(tt.key, rel); err != nil {
			t.Fatalf("failed to create release with key %s: %v", tt.key, err)
		}
	}

	// list all deleted releases
	del, err := gcsDriver.List(func(rel *rspb.Release) bool {
		return rel.Info.Status == rspb.StatusUninstalled
	})
	// check
	if err != nil {
		t.Errorf("Failed to list deleted: %v", err)
	}
	if len(del) != 2 {
		t.Errorf("Expected 2 deleted, got %d:\n%v\n", len(del), del)
	}

	// list all deployed releases
	dpl, err := gcsDriver.List(func(rel *rspb.Release) bool {
		return rel.Info.Status == rspb.StatusDeployed
	})
	// check
	if err != nil {
		t.Errorf("Failed to list deployed: %v", err)
	}
	if len(dpl) != 2 {
		t.Errorf("Expected 2 deployed, got %d:\n%+v\n", len(dpl), dpl)
	}

	// list all superseded releases
	ssd, err := gcsDriver.List(func(rel *rspb.Release) bool {
		return rel.Info.Status == rspb.StatusSuperseded
	})
	// check
	if err != nil {
		t.Errorf("Failed to list superseded: %v", err)
	}
	if len(ssd) != 2 {
		t.Errorf("Expected 2 superseded, got %d:\n%v\n", len(ssd), ssd)
	}
}

func TestGCSQuery(t *testing.T) {
	gcsDriver.pathPrefix = "helm-tests-query"

	namespace := "default"
	tests := []struct {
		key       string
		namespace string
		status    rspb.Status
	}{
		{"gcs-test-list-key-1", namespace, rspb.StatusUninstalled},
		{"gcs-test-list-key-2", namespace, rspb.StatusUninstalled},
		{"gcs-test-list-key-3", namespace, rspb.StatusDeployed},
		{"gcs-test-list-key-4", namespace, rspb.StatusDeployed},
		{"gcs-test-list-key-5", namespace, rspb.StatusSuperseded},
		{"gcs-test-list-key-6", namespace, rspb.StatusSuperseded},
	}

	// create stubs
	for _, tt := range tests {
		rel := releaseStub(tt.key, 1, tt.namespace, tt.status)
		if err := gcsDriver.Create(tt.key, rel); err != nil {
			t.Fatalf("failed to create release with key %s: %v", tt.key, err)
		}
	}

	rls, err := gcsDriver.Query(map[string]string{"status": "deployed"})
	if err != nil {
		t.Fatalf("Failed to query: %s", err)
	}
	if len(rls) != 2 {
		t.Fatalf("Expected 2 results, actual %d", len(rls))
	}

	_, err = gcsDriver.Query(map[string]string{"name": "notExist"})
	if err != ErrReleaseNotFound {
		t.Errorf("Expected {%v}, got {%v}", ErrReleaseNotFound, err)
	}
}

// newTestFixtureGCS mocks the GCS (for testing purposes)
func newTestFixtureGCS() func() {
	flag.Parse()
	prefix := "helm-tests"
	namespace := "default"
	replayFilename := "gcs.replay"

	ctx := context.Background()

	var hc *http.Client
	cleanup := func() {}

	if *gcsRec {
		now := time.Now().UTC()
		if !httpreplay.Supported() {
			panic("HTTP replay not supported")
		}
		nowBytes, err := json.Marshal(now)
		if err != nil {
			panic(err)
		}
		recorder, err := httpreplay.NewRecorder(replayFilename, nowBytes)
		if err != nil {
			panic(err)
		}
		hc, err = recorder.Client(ctx)
		if err != nil {
			panic(err)
		}
		cleanup = func() {
			if err := recorder.Close(); err != nil {
				panic(err)
			}
		}
	} else {
		httpreplay.DebugHeaders()
		replayer, err := httpreplay.NewReplayer(replayFilename)
		if err != nil {
			panic(err)
		}
		var t time.Time
		if err := json.Unmarshal(replayer.Initial(), &t); err != nil {
			panic(err)
		}
		hc, err = replayer.Client(ctx)
		if err != nil {
			panic(err)
		}
	}
	client, _ := storage.NewClient(ctx, option.WithHTTPClient(hc))

	gcsDriver = &GCS{
		client: client,

		bucket:     "helm-tests",
		pathPrefix: prefix,
		namespace:  namespace,

		now: gcsNow,

		Log: func(a string, b ...interface{}) {},
	}

	return cleanup
}
