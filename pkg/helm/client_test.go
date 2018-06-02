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

package helm

import (
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	helmClient := NewClient()
	if helmClient.opts.connectTimeout != 5*time.Second {
		t.Errorf("expected default timeout duration to be 5 seconds, got %v", helmClient.opts.connectTimeout)
	}

	helmClient = NewClient(ConnectTimeout(60))
	if helmClient.opts.connectTimeout != time.Minute {
		t.Errorf("expected timeout duration to be 1 minute, got %v", helmClient.opts.connectTimeout)
	}
}
