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
