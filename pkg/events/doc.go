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

/* Package events provides an event system for Helm.
 *
 * This is not a general purpose event framework. It is a system for handling Helm
 * objects.
 *
 * An event Emitter is an object that binds events and event handlers. An EventHandler
 * is a function that can respond to an event. And a Context is the information
 * passed from the calling location into the event handler. All together, the
 * system works by creating an event emitter, registering event handlers, and
 * then calling the event handlers at the appropriate place and time, passing in
 * a context as you go.
 *
 * In this particular implementation, the Context is considered mutable. So an
 * event handler is allowed to change the contents of the context. It is up to
 * the calling code to decide whether to "accept" those changes back into the
 * calling context.
 */
package events
