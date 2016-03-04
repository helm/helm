/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package util

import (
	"archive/tar"
	"bytes"
	"testing"

	"github.com/ghodss/yaml"
)

const invalidFileName = "afilethatdoesnotexist"

var importFileNames = []string{
	"../test/replicatedservice.py",
}

var (
	testTemplateName       = "expandybird"
	testTemplateType       = "replicatedservice.py"
	testTemplateProperties = `
service_port: 8080
target_port: 8080
container_port: 8080
external_service: true
replicas: 3
image: gcr.io/dm-k8s-prod/expandybird
labels:
  app: expandybird
`
)

func TestNewTemplateFromType(t *testing.T) {
	var properties map[string]interface{}
	if err := yaml.Unmarshal([]byte(testTemplateProperties), &properties); err != nil {
		t.Fatalf("cannot unmarshal test data: %s", err)
	}

	_, err := NewTemplateFromType(testTemplateName, testTemplateType, properties)
	if err != nil {
		t.Fatalf("cannot create template from type %s: %s", testTemplateType, err)
	}
}

func TestNewTemplateFromReader(t *testing.T) {
	r := bytes.NewReader([]byte{})
	if _, err := NewTemplateFromReader("test", r, nil); err == nil {
		t.Fatalf("expected error did not occur for empty input: %s", err)
	}

	r = bytes.NewReader([]byte("test"))
	if _, err := NewTemplateFromReader("test", r, nil); err != nil {
		t.Fatalf("cannot read test template: %s", err)
	}
}

type archiveBuilder []struct {
	Name, Body string
}

var invalidFiles = archiveBuilder{
	{"testFile1.yaml", ""},
}

var validFiles = archiveBuilder{
	{"testFile1.yaml", "testFile:1"},
	{"testFile2.yaml", "testFile:2"},
}

func generateArchive(t *testing.T, files archiveBuilder) *bytes.Reader {
	buffer := new(bytes.Buffer)
	tw := tar.NewWriter(buffer)
	for _, file := range files {
		hdr := &tar.Header{
			Name: file.Name,
			Mode: 0600,
			Size: int64(len(file.Body)),
		}

		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}

		if _, err := tw.Write([]byte(file.Body)); err != nil {
			t.Fatal(err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	r := bytes.NewReader(buffer.Bytes())
	return r
}

func TestNewTemplateFromArchive(t *testing.T) {
	r := bytes.NewReader([]byte{})
	if _, err := NewTemplateFromArchive("", r, nil); err == nil {
		t.Fatalf("expected error did not occur for empty input: %s", err)
	}

	r = bytes.NewReader([]byte("test"))
	if _, err := NewTemplateFromArchive("", r, nil); err == nil {
		t.Fatalf("expected error did not occur for non archive file:%s", err)
	}

	r = generateArchive(t, invalidFiles)
	if _, err := NewTemplateFromArchive(invalidFiles[0].Name, r, nil); err == nil {
		t.Fatalf("expected error did not occur for empty file in archive")
	}

	r = generateArchive(t, validFiles)
	if _, err := NewTemplateFromArchive("", r, nil); err == nil {
		t.Fatalf("expected error did not occur for missing file in archive")
	}

	r = generateArchive(t, validFiles)
	if _, err := NewTemplateFromArchive(validFiles[1].Name, r, nil); err != nil {
		t.Fatalf("cannnot create template from valid archive")
	}
}

func TestNewTemplateFromFileNames(t *testing.T) {
	if _, err := NewTemplateFromFileNames(invalidFileName, importFileNames); err == nil {
		t.Fatalf("expected error did not occur for invalid template file name")
	}

	_, err := NewTemplateFromFileNames(invalidFileName, []string{"afilethatdoesnotexist"})
	if err == nil {
		t.Fatalf("expected error did not occur for invalid import file names")
	}
}
