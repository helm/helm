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

package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientGetWaiterWithOptions_OrderedWaitStrategy(t *testing.T) {
	client := newTestClient(t)

	waiter, err := client.GetWaiterWithOptions(OrderedWaitStrategy)
	require.NoError(t, err)
	assert.IsType(t, &statusWaiter{}, waiter)
}

func TestClientGetWaiterWithOptions_UnknownStrategyListsOrdered(t *testing.T) {
	client := newTestClient(t)

	waiter, err := client.GetWaiterWithOptions(WaitStrategy("bogus"))
	require.Error(t, err)
	assert.Nil(t, waiter)
	assert.Contains(t, err.Error(), string(OrderedWaitStrategy))
}
