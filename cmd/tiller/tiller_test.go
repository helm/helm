package main

import (
	"testing"

	"k8s.io/helm/cmd/tiller/environment"
	"k8s.io/helm/pkg/engine"
	"k8s.io/helm/pkg/storage"
)

// These are canary tests to make sure that the default server actually
// fulfills its requirements.
var _ environment.Engine = &engine.Engine{}
var _ environment.ReleaseStorage = storage.NewMemory()

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
