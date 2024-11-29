package nodesmonitor

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type setupHelper struct {
	obj []client.Object
}

func SetupHelper() *setupHelper {
	return &setupHelper{}
}

func (sh setupHelper) Build() []client.Object {
	return sh.obj
}

func (sh *setupHelper) ControlPlanes(n int) *setupHelper {
	for i := 0; i < n; i++ {
		sh.obj = append(sh.obj, Node(fmt.Sprintf("control-plane-%d", i)).Label("node-role.kubernetes.io/control-plane").Object())
	}
	return sh
}

func (sh *setupHelper) Workers(n int) *setupHelper {
	for i := 0; i < n; i++ {
		sh.obj = append(sh.obj, Node(fmt.Sprintf("worker-%d", i)).Label("node-role.kubernetes.io/worker").Object())
	}
	return sh
}

type nodeBuilder struct {
	object *corev1.Node
}

func Node(name string) *nodeBuilder {
	return &nodeBuilder{
		object: &corev1.Node{
			ObjectMeta: v1.ObjectMeta{
				Name:   name,
				Labels: make(map[string]string),
			},
		},
	}
}

func (nb *nodeBuilder) Label(labelID string, labelValue ...string) *nodeBuilder {
	value := ""
	if len(labelValue) > 0 {
		value = labelValue[0]
	}
	nb.object.ObjectMeta.Labels[labelID] = value
	return nb
}

func (nb *nodeBuilder) Object() client.Object {
	return nb.object
}

func (nb *nodeBuilder) Build() []client.Object {
	return []client.Object{nb.object}
}

type nodesMonitorBuilder struct {
	object *NodesMonitor
}

func NodesMonitorObject(name string, namespace ...string) *nodesMonitorBuilder {
	ns := testNS
	if len(namespace) > 0 {
		ns = namespace[0]
	}

	return &nodesMonitorBuilder{
		object: &NodesMonitor{
			ObjectMeta: v1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: NodesMonitorSpec{
				NodeLabelFilter: "",
				AlertThreshold:  -1,
			},
			Status: NodesMonitorStatus{},
		},
	}
}

func (nmb *nodesMonitorBuilder) Active() *nodesMonitorBuilder {
	nmb.object.Spec.Active = true
	return nmb
}

func (nmb *nodesMonitorBuilder) Filter(filter string) *nodesMonitorBuilder {
	nmb.object.Spec.NodeLabelFilter = filter
	return nmb
}

func (nmb *nodesMonitorBuilder) AlertThreshold(threshold int) *nodesMonitorBuilder {
	nmb.object.Spec.AlertThreshold = threshold
	return nmb
}

func (nmb *nodesMonitorBuilder) Object() client.Object {
	return nmb.object
}

func (nmb *nodesMonitorBuilder) Build() []client.Object {
	return []client.Object{nmb.object}
}
