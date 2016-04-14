package main

import (
	"testing"

	"github.com/deis/tiller/cmd/tiller/environment"
	"github.com/deis/tiller/pkg/engine"
	"github.com/deis/tiller/pkg/storage"
)

// These are canary tests to make sure that the default server actually
// fulfills its requirements.
var _ environment.Engine = &engine.Engine{}
var _ environment.ReleaseStorage = storage.NewMemory()

func TestNewServer(t *testing.T) {
	defer func() {
		if recover() != nil {
			t.Fatalf("Panic trapped. Check EngineYard.Default()")
		}
	}()
	s := newServer()

	// This will panic if it is not correct.
	s.Environment.EngineYard.Default()

	e, ok := s.Environment.EngineYard.Get(environment.GoTplEngine)
	if !ok {
		t.Fatalf("Could not find GoTplEngine")
	}
	if e == nil {
		t.Fatalf("Template engine GoTplEngine returned nil.")
	}
}
