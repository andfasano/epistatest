package nodesmonitor

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "NodesMonitor", Version: "v1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(&NodesMonitor{})
}

type NodesMonitor struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodesMonitorSpec   `json:"spec,omitempty"`
	Status NodesMonitorStatus `json:"status,omitempty"`
}

type NodesMonitorSpec struct {
	// Active indicates whether the monitor must be enabled or not.
	Active bool `json:"active"`

	// AlertThreshold specifies the threshold for triggering an alert.
	AlertThreshold int `json:"alertThreshold"`

	// NodeLabelFilter selects only nodes with the specified label name
	// (value is ignored).
	NodeLabelFilter string `json:"nodeLabelFilter,omitempty"`
}

type NodesMonitorStatus struct {
	// The current number of nodes found.
	NumNodes int `json:"numNodes"`

	// If the current resource is active or not.
	Active bool `json:"active"`

	// History of the threshold changes.
	Conditions []v1.Condition `json:"conditions,omitempty"`
}

func (in *NodesMonitor) GetObjectKind() schema.ObjectKind {
	return &in.TypeMeta
}

func (in *NodesMonitor) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *NodesMonitor) DeepCopy() *NodesMonitor {
	if in == nil {
		return nil
	}
	out := new(NodesMonitor)
	in.DeepCopyInto(out)
	return out
}

func (in *NodesMonitor) DeepCopyInto(out *NodesMonitor) {
	if in == nil {
		return
	}
	*out = *in
	out.ObjectMeta = *in.ObjectMeta.DeepCopy()
	out.Spec = in.Spec
	out.Status = *in.Status.DeepCopy()
}

func (nms *NodesMonitorStatus) AddThresholdExceededCondition(thresholdExceeded bool) {
	newCond := v1.Condition{
		Type:               "ThresholdExceeded",
		Status:             v1.ConditionFalse,
		Message:            "Nodes count is below the threshold.",
		LastTransitionTime: v1.Now(),
	}
	if thresholdExceeded {
		newCond.Status = v1.ConditionTrue
		newCond.Message = "Nodes count is above the threshold."
	}
	lastCond := nms.GetLatestCondition()

	if lastCond == nil || newCond.Status != lastCond.Status {
		nms.Conditions = append(nms.Conditions, newCond)
	}
}

func (nms *NodesMonitorStatus) GetLatestCondition() *v1.Condition {
	if len(nms.Conditions) == 0 {
		return nil
	}

	return &nms.Conditions[len(nms.Conditions)-1]
}

func (in *NodesMonitorStatus) DeepCopy() *NodesMonitorStatus {
	if in == nil {
		return nil
	}
	out := new(NodesMonitorStatus)
	out.NumNodes = in.NumNodes
	out.Active = in.Active
	for _, c := range in.Conditions {
		out.Conditions = append(out.Conditions, v1.Condition{
			Type:               c.Type,
			Status:             c.Status,
			Reason:             c.Reason,
			Message:            c.Message,
			LastTransitionTime: *c.LastTransitionTime.DeepCopy(),
		})
	}
	return out
}
