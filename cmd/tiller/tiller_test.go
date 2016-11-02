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

package main

import (
	"testing"

	"k8s.io/helm/pkg/engine"
	"k8s.io/helm/pkg/tiller/environment"
)

// These are canary tests to make sure that the default server actually
// fulfills its requirements.
var _ environment.Engine = &engine.Engine{}

func TestInit(t *testing.T) {
	defer func() {
		if recover() != nil {
			t.Fatalf("Panic trapped. Check EngineYard.Default()")
		}
	}()

	// This will panic if it is not correct.
	env.EngineYard.Default()

	e, ok := env.EngineYard.Get(environment.GoTplEngine)
	if !ok {
		t.Fatalf("Could not find GoTplEngine")
	}
	if e == nil {
		t.Fatalf("Template engine GoTplEngine returned nil.")
	}
}
