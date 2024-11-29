package epistatest

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rt "k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type testCase struct {
	name          string
	testCase      Testable
	expectedError string
}

func TestScenario(t *testing.T) {
	cases := []testCase{
		{
			name: "minimal",
			testCase: newTestScenario().
				Setup(emptySetup{}).
				ReconcileUntil(func(client client.Client, obj *corev1.ConfigMap) bool {
					return true
				}),
		},
		{
			name: "at least one step is required",
			testCase: newTestScenario().
				Setup(emptySetup{}),
			expectedError: "no steps found",
		},
		{
			name: "next request",
			testCase: newTestScenario().
				Setup(testScenarioBuilder{}).
				NextRequest("cm0", "cm").
				ReconcileUntil(func(client client.Client, obj *corev1.ConfigMap) bool {
					return obj.Name == "cm0"
				}, "wait for cm0").
				NextRequest("cm1", "cm").
				ReconcileUntil(func(client client.Client, obj *corev1.ConfigMap) bool {
					return obj.Name == "cm1"
				}, "wait for cm1"),
		},
		{
			name: "next request object",
			testCase: newTestScenario().
				Setup(emptySetup{}).
				NextRequestObject(func() client.Object {
					return &corev1.ConfigMap{
						ObjectMeta: v1.ObjectMeta{
							Name:      "new-cm",
							Namespace: "cm",
						},
					}
				}).
				ReconcileUntil(func(client client.Client, obj *corev1.ConfigMap) bool {
					return obj.Name == "new-cm"
				}),
		},
		{
			name: "simulate event",
			testCase: newTestScenario().
				Setup(testScenarioBuilder{}).
				NextRequest("cm0", "cm").
				ReconcileUntil(func(client client.Client, obj *corev1.ConfigMap) bool {
					return len(obj.Data) == 0
				}).
				Then(func(client client.Client, obj *corev1.ConfigMap) {
					obj.Data = map[string]string{
						"field": "someValue",
					}
					if err := client.Update(context.Background(), obj); err != nil {
						t.Fatal(err)
					}
				}).
				ReconcileUntil(func(client client.Client, obj *corev1.ConfigMap) bool {
					v, ok := obj.Data["field"]
					return ok && v == "someValue"
				}),
		},
		{
			name: "unsatisfied condition",
			testCase: newTestScenario().
				Setup(testScenarioBuilder{}).
				ReconcileUntil(func(client client.Client, obj *corev1.ConfigMap) bool {
					return false
				}, "expected to fail"),
			expectedError: "`expected to fail` not satisfied, too many reconcile loops (20)",
		},
		{
			name: "max reconciles",
			testCase: newTestScenario().
				WithMaxReconciles(5).
				Setup(testScenarioBuilder{}).
				ReconcileUntil(func(client client.Client, obj *corev1.ConfigMap) bool {
					return false
				}),
			expectedError: "`waiting condition #0` not satisfied, too many reconcile loops (5)",
		},
		{
			name: "error during next request object creation",
			testCase: newTestScenario().
				Setup(emptySetup{}).
				NextRequestObject(func() client.Object {
					return &corev1.ConfigMap{}
				}),
			expectedError: "step `` failure:  \"\" is invalid: metadata.name: Required value: name is required",
		},
		{
			name: "different type on next request",
			testCase: newTestScenario().
				SetupObjects(func() []client.Object {
					return []client.Object{
						&corev1.Node{
							ObjectMeta: v1.ObjectMeta{
								Name: "node",
							},
						},
					}
				}).
				NextRequest("node").
				ReconcileUntil(func(client client.Client, obj *corev1.ConfigMap) bool {
					return obj.Name == ""
				}),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testScenario[TestController, *corev1.ConfigMap](t, tc)
		})
	}
}

func TestScenarioWithWrongControllerType(t *testing.T) {
	cases := []testCase{
		{
			name: "client field required",
			testCase: New[TestControllerWithoutClient, *corev1.ConfigMap]().
				WithSchemes(corev1.AddToScheme).
				Setup(emptySetup{}).
				ReconcileUntil(func(client client.Client, obj *corev1.ConfigMap) bool {
					return true
				}),
			expectedError: "field 'Client' not found for type TestControllerWithoutClient",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testScenario[TestControllerWithoutClient, *corev1.ConfigMap](t, tc)
		})
	}
}

func TestScenarioObjectWithStatus(t *testing.T) {
	cases := []testCase{
		{
			name: "object with status",
			testCase: newTestScenarioWithStatus().
				Setup(emptySetup{}).
				NextRequestObject(func() client.Object {
					return &corev1.Node{
						ObjectMeta: v1.ObjectMeta{
							Name: "node-0",
						},
						Status: corev1.NodeStatus{
							Phase: corev1.NodeRunning,
						},
					}
				}).
				ReconcileUntil(func(client client.Client, obj *corev1.Node) bool {
					return obj.Status.Phase == corev1.NodeRunning
				}),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testScenario[TestController, *corev1.Node](t, tc)
		})
	}
}

func TestMinimal(t *testing.T) {
	New[TestController, *corev1.Node]().
		WithSchemes(corev1.AddToScheme).
		SetupObjects(func() []client.Object {
			return []client.Object{&corev1.Node{
				ObjectMeta: v1.ObjectMeta{Name: "my-node"},
			}}
		}).
		NextRequest("my-node").
		ReconcileUntil(func(client client.Client, node *corev1.Node) bool {
			return node.Name == "my-node"
		}).
		Test(t)
}

type TestControllerWithTerminalError struct {
	client.Client
	Scheme *rt.Scheme
}

func (s TestControllerWithTerminalError) Reconcile(context.Context, ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("unrecoverable error"))
}

func TestReconcileTerminalError(t *testing.T) {
	cases := []testCase{
		{
			name: "unrecoverableError",
			testCase: New[TestControllerWithTerminalError, *corev1.ConfigMap]().
				WithSchemes(corev1.AddToScheme).
				Setup(testScenarioBuilder{}).
				NextRequest("cm-0", "cm").
				ReconcileUntil(func(client client.Client, obj *corev1.ConfigMap) bool {
					return true
				}),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testScenario[TestControllerWithTerminalError, *corev1.ConfigMap](t, tc)
		})
	}
}

func testScenario[R reconcile.Reconciler, T client.Object](t *testing.T, tc testCase) {
	t.Helper()
	s, ok := tc.testCase.(*scenario[R, T])
	if !ok {
		t.FailNow()
	}
	err := s.test()
	if err == nil && tc.expectedError != "" {
		t.Fatalf("expecting error `%s` but none received", tc.expectedError)
	}
	if err != nil && err.Error() != tc.expectedError {
		t.Fatalf("expected error: `%s`, but received `%s", tc.expectedError, err.Error())
	}
}

type TestController struct {
	client.Client
	Scheme *rt.Scheme
}

func (s TestController) Reconcile(context.Context, ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func newTestScenario() Scenario[TestController, *corev1.ConfigMap] {
	return New[TestController, *corev1.ConfigMap]().
		WithSchemes(corev1.AddToScheme)
}

func newTestScenarioWithStatus() Scenario[TestController, *corev1.Node] {
	return New[TestController, *corev1.Node]().
		WithSchemes(corev1.AddToScheme)
}

type testScenarioBuilder struct{}

func (tsb testScenarioBuilder) Build() []client.Object {
	return []client.Object{
		&corev1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      "cm0",
				Namespace: "cm",
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      "cm1",
				Namespace: "cm",
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      "cm2",
				Namespace: "cm",
			},
		},
	}
}

type emptySetup struct{}

func (es emptySetup) Build() []client.Object {
	return nil
}

type TestControllerWithoutClient struct {
	// no client defined here
	Scheme *rt.Scheme
}

func (s TestControllerWithoutClient) Reconcile(context.Context, ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}
