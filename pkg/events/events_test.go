/*
Copyright 2018 The Kubernetes Authors All rights reserved.

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

package events

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func stubFunc(*Context) error {
	return nil
}

func stubErrFunc(*Context) error {
	return errors.New("nope")
}

var _ EventHandler = stubFunc
var _ EventHandler = stubErrFunc

var stubCtx = &Context{}

func TestEmitter(t *testing.T) {
	emitter := New()
	is := assert.New(t)

	is.Len(emitter.Events(), 0, "expect no events")
	is.Equal(0, emitter.Len("foo"), "expect empty foo event")
}

func TestEmitter_On(t *testing.T) {
	emitter := New()
	is := assert.New(t)

	emitter.On("foo", stubFunc)
	is.Equal([]string{"foo"}, emitter.Events(), "expect foo event")
	is.Equal(1, emitter.Len("foo"), "expect one foo event handler")

	emitter.On("foo", stubErrFunc)
	is.Equal(2, emitter.Len("foo"), "expect two foo event handlers")

	emitter.On("bar", stubFunc)
	is.Equal([]string{"foo", "bar"}, emitter.Events(), "expect foo, bar events")
	is.Equal(1, emitter.Len("bar"), "expect one bar event handler")
}

func TestEmitter_Handlers(t *testing.T) {
	emitter := New()
	is := assert.New(t)

	is.Len(emitter.Handlers("foo"), 0, "expect no handlers")

	emitter.On("foo", stubFunc)
	is.Len(emitter.Handlers("foo"), 1, "expect one handler")

	emitter.On("foo", stubFunc)
	emitter.On("foo", stubErrFunc)

	is.Len(emitter.Handlers("foo"), 3, "expect 3 handlers")

	fn := emitter.Handlers("foo")[2]
	is.Error(fn(stubCtx), "expect last one to be the stubErrFunc")
}

func TestEmitter_SetHandlers(t *testing.T) {
	emitter := New()
	is := assert.New(t)

	emitter.On("foo", stubFunc)
	emitter.SetHandlers("foo", []EventHandler{stubErrFunc})
	is.Equal(emitter.Len("foo"), 1, "Expect one handler")

	fn := emitter.Handlers("foo")[0]
	is.Error(fn(stubCtx), "expect only handler to be stubErrFunc")
}

func TestEmitter_Emit(t *testing.T) {
	emitter := New()
	is := assert.New(t)

	emitter.On("foo", stubFunc)
	is.Nil(emitter.Emit("foo", stubCtx), "Expect no error")

	emitter.On("foo", stubErrFunc)
	is.Error(emitter.Emit("foo", stubCtx), "Expect error")
}
