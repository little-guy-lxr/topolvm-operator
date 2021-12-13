package csi

type Param struct {
	RawDeviceImage               string
	RegistrarImage               string
	ProvisionerImage             string
	AttacherImage                string
	SnapshotterImage             string
	LivenessImage                string
	ResizerImage                 string
	DriverNamePrefix             string
	KubeletDirPath               string
	LogLevel                     uint8
	PluginPriorityClassName      string
	ProvisionerPriorityClassName string
	ProvisionerReplicas          int32
	TopolvmImage                 string
}

type TemplateParam struct {
	Param
	// non-global template only parameters
	Namespace string
}
