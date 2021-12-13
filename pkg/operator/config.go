package operator

type OperatorConfig struct {
	OperatorNamespace string
	Image             string
	ServiceAccount    string
	NamespaceToWatch  string
	Parameters        map[string]string
}

const (
	// OperatorSettingConfigMapName refers to ConfigMap that configures rook ceph operator
	OperatorSettingConfigMapName string = "topolvm-operator-config"
	EnableRawDeviceEnv           string = "ENABLE_RAW_DEVICE"
	DiscoverAppName              string = "discover-device"
)
