package nodesmonitor

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NodesMonitorController struct {
	client.Client
	Scheme *runtime.Scheme
}

func (c NodesMonitorController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	nm := &NodesMonitor{}
	if err := c.Get(ctx, req.NamespacedName, nm); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Check for enabling or disabling monitoring.
	if nm.Spec.Active != nm.Status.Active {
		nm.Status.Active = nm.Spec.Active
		if err := c.Status().Update(ctx, nm); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// If enabled, let's count the current nodes.
	if nm.Status.Active {
		nodes, err := c.listFilteredNodes(ctx, nm)
		if err != nil {
			return ctrl.Result{}, err
		}

		// The current number of nodes changed, so let's update accordingly the resource status.
		if len(nodes) != nm.Status.NumNodes {
			nm.Status.NumNodes = len(nodes)
			nm.Status.AddThresholdExceededCondition(nm.Status.NumNodes >= nm.Spec.AlertThreshold)

			if err := c.Status().Update(ctx, nm); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

func (c NodesMonitorController) listFilteredNodes(ctx context.Context, nm *NodesMonitor) ([]corev1.Node, error) {
	nodeList := &corev1.NodeList{}
	listOptions := &client.ListOptions{}

	// If a filter was defined, let's apply it.
	if nm.Spec.NodeLabelFilter != "" {
		labelSelector := labels.NewSelector()
		requirement, err := labels.NewRequirement(nm.Spec.NodeLabelFilter, selection.Exists, nil)
		if err != nil {
			return nil, err
		}
		labelSelector = labelSelector.Add(*requirement)
		listOptions.LabelSelector = labelSelector
	}

	// Get the nodes.
	if err := c.List(ctx, nodeList, listOptions); err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	return nodeList.Items, nil
}
