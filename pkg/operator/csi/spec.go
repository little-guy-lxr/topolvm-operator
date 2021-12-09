package csi

type Param struct {
	RegistrarImage               string
	ProvisionerImage             string
	AttacherImage                string
	SnapshotterImage             string
	ResizerImage                 string
	DriverNamePrefix             string
	KubeletDirPath               string
	LogLevel                     uint8
	CephFSGRPCMetricsPort        uint16
	PluginPriorityClassName      string
	ProvisionerPriorityClassName string
	ProvisionerReplicas          int32
	EnableRawDeviceDriver        bool
}

type TemplateParam struct {
	Param
	// non-global template only parameters
	Namespace string
}
