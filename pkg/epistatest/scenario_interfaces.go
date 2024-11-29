package epistatest

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// New creates a new Scenario instance for the configured reconciler and
// client object types.
func New[R reconcile.Reconciler, T client.Object]() Scenario[R, T] {
	return newScenario[R, T]()
}

// Scenario represents the entry point for a fluent interface
// suitable for writing a new test against the configured reconciler type R.
// The client object type T refers instead the main resource type handled
// by the reconciler.
type Scenario[R reconcile.Reconciler, T client.Object] interface {
	// Allows to register multiple schemes for the current scenario.
	WithSchemes(...func(s *runtime.Scheme) error) Scenario[R, T]
	// Defines the maximum number of reconciles loops that will be
	// executed continuosly before declaring a failure when testing
	// a reconcile condition (see ReconcileUntil).
	WithMaxReconciles(n int) Scenario[R, T]
	// This method can be used to feed a number of initial objects
	// in the current scenario.
	Setup(...ObjectsBuilder) _reconcileNextRequest[T]
	// Same as Setup, but using directly an inline function.
	SetupObjects(func() []client.Object) _reconcileNextRequest[T]
}

// ObjectsBuilder is a convenient interface for creating helpers to
// generate a list of object for setting up a scenario.
type ObjectsBuilder interface {
	// Build method will be invoked by the Setup to create the list
	// of initial objects.
	Build() []client.Object
}

type _reconcileNextRequest[T client.Object] interface {
	_reconcileLoop[T]
	// NextRequest specifies which resource will be triggered for the next
	// reconcile invocations. The resource must be already present in the
	// current cache.
	NextRequest(name string, namespace ...string) _reconcileLoop[T]
	// Similar to NextRequest, but it allows to create a new client object.
	// Useful when the object is not already present in the cache.
	NextRequestObject(func() client.Object) _reconcileLoop[T]
}

type _reconcileLoop[T client.Object] interface {
	_reconcileLeaf[T]
	// This method will keep invoking the reconciler until the predicate will be
	// satisfied, or will make the test fail if the number of unsuccesfull reconciles
	// will be equal or greater than the configured max reconciles value (default: 20).
	// An additional label can be optionally specified, to make it easier identifying
	// the step in case of failure.
	// The predicate will be provided with a client instace, and the current reconcile
	// object (if it was matching the configured object type T).
	ReconcileUntil(f func(client client.Client, obj T) bool, labels ...string) _reconcileAction[T]
}

type _reconcileAction[T client.Object] interface {
	_reconcileNextRequest[T]
	// The Then allows to specify an handler usually invoked after a successfull ReconcileUntil.
	// The handler could be used to modify the current environment.
	Then(f func(client client.Client, obj T), labels ...string) _reconcileNextRequest[T]
}

type _reconcileLeaf[T client.Object] interface {
	Testable
	Case() Testable
}

// For test environment integration.
type Testable interface {
	Test(t *testing.T)
}
