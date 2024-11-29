package epistatest

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	defaultMaxReconciles = 20
)

type scenario[R reconcile.Reconciler, T client.Object] struct {
	maxReconciles int // max number of reconcile steps

	schemes []func(*runtime.Scheme) error // list of schemes to be applied
	setup   func() []client.Object        // setup handler
	steps   []reconcileStep[T]            // steps to be executed

	reconciler reconcile.Reconciler // user reconciler
	client     client.WithWatch     // client to be used with the reconciler
}

type reconcileStep[T runtime.Object] struct {
	label string

	waitFor func(client client.Client, obj T) bool
	action  func(client client.Client, obj T)
	nextReq func() (types.NamespacedName, error)
}

func newScenario[R reconcile.Reconciler, T client.Object]() *scenario[R, T] {
	return &scenario[R, T]{
		maxReconciles: defaultMaxReconciles,
	}
}

func (s *scenario[R, T]) WithSchemes(schemes ...func(s *runtime.Scheme) error) Scenario[R, T] {
	s.schemes = append(s.schemes, schemes...)
	return s
}

func (s *scenario[R, T]) WithMaxReconciles(n int) Scenario[R, T] {
	s.maxReconciles = n
	return s
}

func (s *scenario[R, T]) SetupObjects(setup func() []client.Object) _reconcileNextRequest[T] {
	s.setup = setup
	return s
}

func (s *scenario[R, T]) Setup(builders ...ObjectsBuilder) _reconcileNextRequest[T] {
	return s.SetupObjects(func() []client.Object {
		var objs []client.Object
		for _, b := range builders {
			objs = append(objs, b.Build()...)
		}
		return objs
	})
}

func (s *scenario[R, T]) NextRequest(name string, namespace ...string) _reconcileLoop[T] {
	nextReq := func() (types.NamespacedName, error) {
		r := types.NamespacedName{
			Name: name,
		}
		if len(namespace) > 0 {
			r.Namespace = namespace[0]
		}
		return r, nil
	}

	s.steps = append(s.steps, reconcileStep[T]{
		nextReq: nextReq,
	})
	return s
}

func (s *scenario[R, T]) NextRequestObject(nextReqObj func() client.Object) _reconcileLoop[T] {
	nextReq := func() (types.NamespacedName, error) {
		obj := nextReqObj()

		// Create the object.
		if err := s.client.Create(context.Background(), obj, &client.CreateOptions{}); err != nil {
			return types.NamespacedName{}, err
		}

		// Update it with the status (if present), as needed by the fake client.
		if err := s.client.Status().Update(context.Background(), obj); err != nil && !k8serr.IsNotFound(err) {
			return types.NamespacedName{}, err
		}

		r := types.NamespacedName{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		}
		return r, nil
	}

	s.steps = append(s.steps, reconcileStep[T]{
		nextReq: nextReq,
	})
	return s
}

func (s *scenario[R, T]) ReconcileUntil(waitFor func(client client.Client, obj T) bool, labels ...string) _reconcileAction[T] {
	s.steps = append(s.steps, reconcileStep[T]{
		waitFor: waitFor,
		label:   strings.Join(labels, ", "),
	})
	return s
}

func (s *scenario[R, T]) Then(action func(client client.Client, obj T), labels ...string) _reconcileNextRequest[T] {
	lastStep := &s.steps[len(s.steps)-1]
	lastStep.action = action
	lastStep.label = strings.Join(labels, ", ")
	return s
}

func (s *scenario[R, T]) Case() Testable {
	return s
}

func (s *scenario[R, T]) makeScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()

	for _, addToScheme := range s.schemes {
		if err := addToScheme(scheme); err != nil {
			return nil, err
		}
	}

	return scheme, nil
}

func (s *scenario[R, T]) newObjectInstance() T {
	return reflect.New(reflect.TypeOf(*new(T)).Elem()).Interface().(T)
}

func (s *scenario[R, T]) Test(t *testing.T) {
	t.Helper()
	if err := s.test(); err != nil {
		t.Fatal(err)
	}
}

func (s *scenario[R, T]) test() error {
	if err := s.setupEnv(); err != nil {
		return err
	}
	return s.run()
}

func (s *scenario[R, T]) setupEnv() error {
	if len(s.steps) == 0 {
		return fmt.Errorf("no steps found")
	}

	scheme, err := s.makeScheme()
	if err != nil {
		return err
	}

	objs := s.setup()
	s.client = fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(objs...).
		Build()

	reconciler, err := s.createReconcilerWithClient()
	if err != nil {
		return err
	}
	s.reconciler = reconciler

	return nil
}

func (s *scenario[R, T]) reconcileStepError(step reconcileStep[T], err error) error {
	return fmt.Errorf("step `%s` failure: %w", step.label, err)
}

func (s *scenario[R, T]) run() error {
	var nextReq types.NamespacedName
	var err error

	for idx, step := range s.steps {
		// Prepare the object for the next reconcile invokation.
		if step.nextReq != nil {
			nextReq, err = step.nextReq()
			if err != nil {
				return s.reconcileStepError(step, err)
			}
			continue
		}

		// Keep reconciling until either the waitFor condition will be satisfied or max reconcile
		// steps will be reached.
		// In addition, like for the regular controller-runtime case, a TerminalError will stop
		// the reconciliation.
		reconcileCounter := 0
		for ; reconcileCounter < s.maxReconciles; reconcileCounter++ {
			_, err = s.reconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: nextReq})
			if errors.Is(err, reconcile.TerminalError(nil)) {
				return nil
			}

			latestUpdatedObj := s.newObjectInstance()
			err = s.client.Get(context.Background(), nextReq, latestUpdatedObj)
			if err != nil && !k8serr.IsNotFound(err) {
				return s.reconcileStepError(step, err)
			}

			if step.waitFor(s.client, latestUpdatedObj) {
				if step.action != nil {
					step.action(s.client, latestUpdatedObj)
				}
				reconcileCounter = 0
				break
			}
		}

		if reconcileCounter >= s.maxReconciles {
			label := step.label
			if label == "" {
				label = fmt.Sprintf("waiting condition #%s", strconv.Itoa(idx))
			}
			return fmt.Errorf("`%s` not satisfied, too many reconcile loops (%d)", label, s.maxReconciles)
		}
	}

	return nil
}

func (s *scenario[R, T]) createReconcilerWithClient() (reconcile.Reconciler, error) {
	reconciler := new(R)

	fv := reflect.ValueOf(reconciler).Elem().FieldByName("Client")
	if !fv.IsValid() {
		return nil, fmt.Errorf("field 'Client' not found for type %s", reflect.TypeOf(reconciler).Elem().Name())
	}

	fv.Set(reflect.ValueOf(s.client))
	return *reconciler, nil
}
