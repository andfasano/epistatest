# Epistatest
A tiny, lightweight GO framework to support writing and maintaing unit tests for Kubernetes controllers.

## Goals
* Easy to use - and read - fluent interfaces for writing unit tests on controllers
    * Hide all the boilerplate code 
    * Promote the builder pattern
* Full, fine-grained control on the sequence of events
* Lightweight environment, integration with the native GO testing API

## Non-goals
* Support e2e or integration tests.
* A complete controller-runtime mock.

## Getting started

The following short example demonstrates how to test an illustrative k8s controller named `MyNodeController`,
which is designed to apply a `my-node` label whenever a new Node is created.

```go
import (
	"github.com/andfasano/epistatest/pkg/epistatest"
)

func TestNodeLabeling(t *testing.T) {
	epistatest.New[MyNodeController, *corev1.Node]().
		WithSchemes(corev1.AddToScheme).
		SetupObjects(func() []client.Object {
			return []client.Object{&corev1.Node{
				ObjectMeta: v1.ObjectMeta{Name: "my-node"},
			}}
		}).
		NextRequest("my-node").
		ReconcileUntil(func(client client.Client, node *corev1.Node) bool {
			_, found := node.Labels["my-label"]
			return found
		}, "wait for the controller to set the label").
		Test(t)
}
```

## FAQ
* Why not using [envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest)?
  envtest may be more suitable for integration testing, since it spins up a minimal - but fully working - control plane,
  composed by the api-server and etcd binaries. 
  Another useful feature from Epistatest, not available in envtest, it's the ability to precisely control the sequence
  of events, something not easily achievable in a live environment.

## Examples
Check [./examples](./examples) directory content for some use cases on how to use the framework.

