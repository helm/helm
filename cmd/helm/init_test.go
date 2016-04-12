package main

import (
	"testing"
)

func TestRunInit(t *testing.T) {

	//TODO: call command and make sure no error is recevied
	err := RunInit(initCmd, nil)
	if err != nil {
		t.Errorf("Expected no error but got one: %s", err)
	}
}
