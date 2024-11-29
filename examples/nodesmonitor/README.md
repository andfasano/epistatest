# NodesMonitor controller tests example

This example shows how to unit test the relevant behaviors of a sample controller, `NodesMonitorController`.
Its main duty is counting the nodes in the cluster, by honoring the configuration specified by the user in the
`NodesMonitor` custom resource.

Different resources could be used to count different kind of nodes. The user can specify a label filter to select 
which nodes must be counted, and an alert threshold value. Every time the number of nodes crosses the threshold, 
a new `ThresholdExceeded` condition is appended in the related resource.
It's also possible to turn on/off the monitoring activity by setting the `Active` field.