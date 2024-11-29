package nodesmonitor

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/andfasano/epistatest/pkg/epistatest"
)

const (
	testNS = "nodes-monitor"
)

func TestNodesMonitorController(t *testing.T) {
	cases := []struct {
		name     string
		testCase epistatest.Testable
	}{
		{
			name: "one node",
			testCase: epistatest.New[NodesMonitorController, *NodesMonitor]().
				WithSchemes(AddToScheme, corev1.AddToScheme).
				Setup(
					Node("node-0"),
					NodesMonitorObject("nodes-counter", testNS).Active()).
				NextRequest("nodes-counter", testNS).
				ReconcileUntil(func(client client.Client, obj *NodesMonitor) bool {
					return obj.Status.NumNodes == 1
				}, "check that exactly one node was found"),
		},
		{
			name: "inactive",
			testCase: epistatest.New[NodesMonitorController, *NodesMonitor]().
				WithSchemes(AddToScheme, corev1.AddToScheme).
				Setup(
					Node("node-0"),
					NodesMonitorObject("nodes-counter", testNS)).
				NextRequest("nodes-counter", testNS).
				ReconcileUntil(func(client client.Client, obj *NodesMonitor) bool {
					return obj.Status.NumNodes == 0 && obj.Status.Active == false
				}, "check initial resource creation").
				Then(func(client client.Client, obj *NodesMonitor) {
					// A user turns on the monitoring on the NodeMonitor resource
					obj.Spec.Active = true
					client.Update(context.Background(), obj)
				}, "enable the control planes monitoring").
				ReconcileUntil(func(client client.Client, obj *NodesMonitor) bool {
					return obj.Status.NumNodes == 1 && obj.Status.Active == true
				}),
		},
		{
			name: "using filter for counting just a subset of nodes",
			testCase: epistatest.New[NodesMonitorController, *NodesMonitor]().
				WithSchemes(AddToScheme, corev1.AddToScheme).
				Setup(
					SetupHelper().ControlPlanes(3).Workers(2),
					NodesMonitorObject("control-plane-counter").Active().Filter("node-role.kubernetes.io/control-plane"),
					NodesMonitorObject("worker-counter").Active().Filter("node-role.kubernetes.io/worker")).
				NextRequest("control-plane-counter", testNS).
				ReconcileUntil(func(client client.Client, obj *NodesMonitor) bool {
					return obj.Name == "control-plane-counter" && obj.Status.NumNodes == 3
				}, "let's check the number of control-planes").
				NextRequest("worker-counter", testNS).
				ReconcileUntil(func(client client.Client, obj *NodesMonitor) bool {
					return obj.Name == "worker-counter" && obj.Status.NumNodes == 2
				}, "then verify the number of workers"),
		},
		{
			name: "going over the threshold",
			testCase: epistatest.New[NodesMonitorController, *NodesMonitor]().
				WithSchemes(AddToScheme, corev1.AddToScheme).
				Setup(
					SetupHelper().ControlPlanes(3).Workers(1),
					NodesMonitorObject("control-plane-counter").Active().Filter("node-role.kubernetes.io/control-plane").AlertThreshold(4)).
				NextRequest("control-plane-counter", testNS).
				ReconcileUntil(func(client client.Client, obj *NodesMonitor) bool {
					return obj.Status.NumNodes == 3 &&
						obj.Status.Conditions[0].Status == v1.ConditionFalse
				}, "initially the number of control plane nodes are below the threshold").
				Then(func(client client.Client, obj *NodesMonitor) {
					client.Create(context.Background(), Node("control-plane-4").Label("node-role.kubernetes.io/control-plane").Object())
				}, "add a new node to reach the threshold").
				ReconcileUntil(func(client client.Client, obj *NodesMonitor) bool {
					return obj.Status.NumNodes == 4 &&
						obj.Status.Conditions[1].Status == v1.ConditionTrue
				}, "verify that the threshold alert has been triggered"),
		},
		{
			name: "falling back below threshold",
			testCase: epistatest.New[NodesMonitorController, *NodesMonitor]().
				WithSchemes(AddToScheme, corev1.AddToScheme).
				Setup(
					SetupHelper().ControlPlanes(3).Workers(1),
					NodesMonitorObject("control-plane-counter").
						Active().
						Filter("node-role.kubernetes.io/control-plane").
						AlertThreshold(3)).
				NextRequest("control-plane-counter", testNS).
				ReconcileUntil(func(client client.Client, obj *NodesMonitor) bool {
					return obj.Status.NumNodes == 3 &&
						obj.Status.GetLatestCondition().Status == v1.ConditionTrue
				}, "wait for the initial status").
				Then(func(client client.Client, obj *NodesMonitor) {
					client.Delete(context.Background(), Node("control-plane-0").Object())
				}, "remove one node to decrease the counter below the threshold").
				ReconcileUntil(func(client client.Client, obj *NodesMonitor) bool {
					cond := obj.Status.GetLatestCondition()
					return obj.Status.NumNodes == 2 &&
						cond.Status == v1.ConditionFalse
				}, "verify the new updated status"),
		},
		{
			name: "avoid redundant conditions updates",
			testCase: epistatest.New[NodesMonitorController, *NodesMonitor]().
				WithSchemes(AddToScheme, corev1.AddToScheme).
				Setup(
					SetupHelper().ControlPlanes(3),
					NodesMonitorObject("control-plane-counter").Active().Filter("node-role.kubernetes.io/control-plane").AlertThreshold(2)).
				NextRequest("control-plane-counter", testNS).
				ReconcileUntil(func(client client.Client, obj *NodesMonitor) bool {
					return obj.Status.NumNodes == 3 &&
						obj.Status.GetLatestCondition().Status == v1.ConditionTrue &&
						len(obj.Status.Conditions) == 1
				}, "wait for the initial status").
				Then(func(client client.Client, obj *NodesMonitor) {
					client.Create(context.Background(), Node("control-plane-4").Label("node-role.kubernetes.io/control-plane").Object())
				}, "add another control-plane node, still above the threshold").
				ReconcileUntil(func(client client.Client, obj *NodesMonitor) bool {
					return obj.Status.NumNodes == 4 &&
						obj.Status.GetLatestCondition().Status == v1.ConditionTrue &&
						len(obj.Status.Conditions) == 1
				}, "verify the counter was updated, but not the conditions"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, tc.testCase.Test)
	}
}
