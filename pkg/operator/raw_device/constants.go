package raw_device

import corev1 "k8s.io/api/core/v1"

// CapacityResource is the resource name of topolvm capacity.
const CapacityResource = corev1.ResourceName("rawdevice.localstor.com/capacity")

// PluginName is the name of the CSI plugin.
const PluginName = "rawdevice.localstor.com"

// TopologyNodeKey is the key of topology that represents node name.
const TopologyNodeKey = "topology.rawdevice.localstor.com/node"

const DefaultCSISocket = "/run/raw-device/csi-rawdevice.sock"
