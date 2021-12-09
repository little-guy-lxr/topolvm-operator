package csi

import (
	_ "embed"
	"fmt"
	"github.com/alauda/topolvm-operator/pkg/operator/csi"
	"github.com/alauda/topolvm-operator/pkg/operator/k8sutil"
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"strconv"
)

var (
	DefaultRawDevicePluginImage = "quay.io/cephcsi/cephcsi:v3.4.0"
	DefaultRegistrarImage       = "k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.3.0"
	DefaultProvisionerImage     = "k8s.gcr.io/sig-storage/csi-provisioner:v3.0.0"
)

const (
	KubeMinMajor                     = "1"
	kubeMinMinor                     = "21"
	defaultLogLevel            uint8 = 0
	provisionerTolerationsEnv        = "CSI_PROVISIONER_TOLERATIONS"
	provisionerNodeAffinityEnv       = "CSI_PROVISIONER_NODE_AFFINITY"
	pluginTolerationsEnv             = "CSI_PLUGIN_TOLERATIONS"
	pluginNodeAffinityEnv            = "CSI_PLUGIN_NODE_AFFINITY"

	rawDevicePluginTolerationsEnv       = "CSI_RAW_DEVICE_PLUGIN_TOLERATIONS"
	rawDevicePluginNodeAffinityEnv      = "CSI_RAW_DEVICE_PLUGIN_NODE_AFFINITY"
	rawDeviceProvisionerTolerationsEnv  = "CSI_RAW_DEVICE_PROVISIONER_TOLERATIONS"
	rawDeviceProvisionerNodeAffinityEnv = "CSI_RAW_DEVICE_PROVISIONER_NODE_AFFINITY"

	rawDeviceProvisionerResource = "CSI_RAW_DEVICE_PROVISIONER_RESOURCE"
	rawDevicePluginResource      = "CSI_RAW_DEVICE_PLUGIN_RESOURCE"
	// default provisioner replicas
	defaultProvisionerReplicas int32 = 2

	csiRawDevicePlugin = "csi-raw-device-plugin"

	csiRawDeviceProvisioner = "csi-raw-device-provisioner"
)

var (
	CSIParam csi.Param

	EnableRawDevice = false
	//driver names
	RawDeviceDriverName string
)

var (
	// Local package template path for RBD
	//go:embed template/csi-rawdevice-plugin.yaml
	CSIRawDeviceNodeTemplatePath string
	//go:embed template/csi-rawdevice-provisioner.yaml
	CSIRawDeviceControllerTemplatePath string
	//go:embed template/raw-device-csi-driver.yaml
	RawDeviceCSIDriverTemplatePath string
)

func (r *CSIRawDeviceController) startDrivers(ver *version.Info, ownerInfo *k8sutil.OwnerInfo) error {
	var (
		err                  error
		rawDevicePlugin      *apps.DaemonSet
		rawDeviceProvisioner *apps.Deployment
		rawDeivceCSIDriver   *storagev1.CSIDriver
	)

	tp := csi.TemplateParam{
		Param:     CSIParam,
		Namespace: r.opConfig.OperatorNamespace,
	}
	// if the user didn't specify a custom DriverNamePrefix use
	// the namespace (and a dot).
	if tp.DriverNamePrefix == "" {
		tp.DriverNamePrefix = fmt.Sprintf("%s.", r.opConfig.OperatorNamespace)
	}

	// default value `system-node-critical` is the highest available priority
	tp.PluginPriorityClassName = k8sutil.GetValue(r.opConfig.Parameters, "CSI_PLUGIN_PRIORITY_CLASSNAME", "")

	// default value `system-cluster-critical` is applied for some
	// critical pods in cluster but less priority than plugin pods
	tp.ProvisionerPriorityClassName = k8sutil.GetValue(r.opConfig.Parameters, "CSI_PROVISIONER_PRIORITY_CLASSNAME", "")

	enableRawDevice := k8sutil.GetValue(r.opConfig.Parameters, "CSI_ENABLE_RAW_DEVICE", "false")

	// if k8s >= v1.17 enable RBD and CephFS snapshotter by default
	if enableRawDevice == "true" && ver.Major == KubeMinMajor && ver.Minor >= kubeMinMinor {
		EnableRawDevice = true
	} else {
		EnableRawDevice = false
	}

	logger.Infof("Kubernetes version is %s.%s", ver.Major, ver.Minor)

	logLevel := k8sutil.GetValue(r.opConfig.Parameters, "CSI_LOG_LEVEL", "")
	tp.LogLevel = defaultLogLevel
	if logLevel != "" {
		l, err := strconv.ParseUint(logLevel, 10, 8)
		if err != nil {
			logger.Errorf("failed to parse CSI_LOG_LEVEL. Defaulting to %d. %v", defaultLogLevel, err)
		} else {
			tp.LogLevel = uint8(l)
		}
	}

	tp.ProvisionerReplicas = defaultProvisionerReplicas
	nodes, err := r.context.Clientset.CoreV1().Nodes().List(r.opManagerContext, metav1.ListOptions{})
	if err == nil {
		if len(nodes.Items) == 1 {
			tp.ProvisionerReplicas = 1
		} else {
			replicas := k8sutil.GetValue(r.opConfig.Parameters, "CSI_PROVISIONER_REPLICAS", "2")
			r, err := strconv.ParseInt(replicas, 10, 32)
			if err != nil {
				logger.Errorf("failed to parse CSI_PROVISIONER_REPLICAS. Defaulting to %d. %v", defaultProvisionerReplicas, err)
			} else {
				tp.ProvisionerReplicas = int32(r)
			}
		}
	} else {
		logger.Errorf("failed to get nodes. Defaulting the number of replicas of provisioner pods to %d. %v", tp.ProvisionerReplicas, err)
	}

	if EnableRawDevice {
		rawDevicePlugin, err = csi.TemplateToDaemonSet("raw-device-node", CSIRawDeviceNodeTemplatePath, tp)
		if err != nil {
			return errors.Wrap(err, "failed to load rbdplugin template")
		}

		rawDeviceProvisioner, err = csi.TemplateToDeployment("raw-device-provisioner", CSIRawDeviceControllerTemplatePath, tp)
		if err != nil {
			return errors.Wrap(err, "failed to load rbd provisioner deployment template")
		}

		rawDeivceCSIDriver, err = csi.TemplateToCSIDriver("raw-device-csi-driver", RawDeviceCSIDriverTemplatePath, tp)
		if err != nil {
			return errors.Wrap(err, "failed to load rbd provisioner deployment template")
		}
	}

	// get common provisioner tolerations and node affinity
	provisionerTolerations := csi.GetToleration(r.opConfig.Parameters, provisionerTolerationsEnv, []corev1.Toleration{})
	provisionerNodeAffinity := csi.GetNodeAffinity(r.opConfig.Parameters, provisionerNodeAffinityEnv, &corev1.NodeAffinity{})
	// get common plugin tolerations and node affinity
	pluginTolerations := csi.GetToleration(r.opConfig.Parameters, pluginTolerationsEnv, []corev1.Toleration{})
	pluginNodeAffinity := csi.GetNodeAffinity(r.opConfig.Parameters, pluginNodeAffinityEnv, &corev1.NodeAffinity{})

	if rawDevicePlugin != nil {
		// get RBD plugin tolerations and node affinity, defaults to common tolerations and node affinity if not specified
		rawDevicePluginTolerations := csi.GetToleration(r.opConfig.Parameters, rawDevicePluginTolerationsEnv, pluginTolerations)
		rawDevicePluginNodeAffinity := csi.GetNodeAffinity(r.opConfig.Parameters, rawDevicePluginNodeAffinityEnv, pluginNodeAffinity)
		// apply RBD plugin tolerations and node affinity
		csi.ApplyToPodSpec(&rawDevicePlugin.Spec.Template.Spec, rawDevicePluginNodeAffinity, rawDevicePluginTolerations)
		// apply resource request and limit to rbdplugin containers
		csi.ApplyResourcesToContainers(r.opConfig.Parameters, rawDevicePluginResource, &rawDevicePlugin.Spec.Template.Spec)
		err = ownerInfo.SetControllerReference(rawDevicePlugin)
		if err != nil {
			return errors.Wrapf(err, "failed to set owner reference to rbd plugin daemonset %q", rawDevicePlugin.Name)
		}
		err = k8sutil.CreateDaemonSet(r.opManagerContext, csiRawDevicePlugin, r.opConfig.OperatorNamespace, r.context.Clientset, rawDevicePlugin)
		if err != nil {
			return errors.Wrapf(err, "failed to start rbdplugin daemonset %q", rawDevicePlugin.Name)
		}
	}

	if rawDeviceProvisioner != nil {
		// get RBD provisioner tolerations and node affinity, defaults to common tolerations and node affinity if not specified
		rawDeviceProvisionerTolerations := csi.GetToleration(r.opConfig.Parameters, rawDeviceProvisionerTolerationsEnv, provisionerTolerations)
		rawDeviceProvisionerNodeAffinity := csi.GetNodeAffinity(r.opConfig.Parameters, rawDeviceProvisionerNodeAffinityEnv, provisionerNodeAffinity)
		// apply RBD provisioner tolerations and node affinity
		csi.ApplyToPodSpec(&rawDeviceProvisioner.Spec.Template.Spec, rawDeviceProvisionerNodeAffinity, rawDeviceProvisionerTolerations)
		// apply resource request and limit to rbd provisioner containers
		csi.ApplyResourcesToContainers(r.opConfig.Parameters, rawDeviceProvisionerResource, &rawDeviceProvisioner.Spec.Template.Spec)
		err = ownerInfo.SetControllerReference(rawDeviceProvisioner)
		if err != nil {
			return errors.Wrapf(err, "failed to set owner reference to rbd provisioner deployment %q", rawDeviceProvisioner.Name)
		}
		antiAffinity := csi.GetPodAntiAffinity("app", csiRawDeviceProvisioner)
		rawDeviceProvisioner.Spec.Template.Spec.Affinity.PodAntiAffinity = &antiAffinity
		rawDeviceProvisioner.Spec.Strategy = apps.DeploymentStrategy{
			Type: apps.RecreateDeploymentStrategyType,
		}
		_, err = k8sutil.CreateOrUpdateDeployment(r.opManagerContext, r.context.Clientset, rawDeviceProvisioner)
		if err != nil {
			return errors.Wrapf(err, "failed to start rbd provisioner deployment %q", rawDeviceProvisioner.Name)
		}
		logger.Info("successfully started CSI Ceph RBD driver")
	}

	if rawDeivceCSIDriver != nil {
		err = k8sutil.CreateCSIDriver(r.opManagerContext, r.context.Clientset, rawDeivceCSIDriver)
		if err != nil {
			return errors.Wrapf(err, "failed to start rbd provisioner deployment %q", rawDeviceProvisioner.Name)
		}
	}

	return nil
}

func (r *CSIRawDeviceController) stopDrivers(ver *version.Info) {
	if !EnableRawDevice {
		logger.Info("CSI Ceph RBD driver disabled")
		succeeded := r.deleteCSIDriverResources(ver, csiRawDevicePlugin, csiRawDeviceProvisioner, "rawdevice.nativestor.io")
		if succeeded {
			logger.Info("successfully removed CSI Ceph RBD driver")
		} else {
			logger.Error("failed to remove CSI Ceph RBD driver")
		}
	}
}

func (r *CSIRawDeviceController) deleteCSIDriverResources(ver *version.Info, daemonset, deployment, driverName string) bool {
	succeeded := true

	err := k8sutil.DeleteDaemonset(r.opManagerContext, r.context.Clientset, r.opConfig.OperatorNamespace, daemonset)
	if err != nil {
		logger.Errorf("failed to delete the %q. %v", daemonset, err)
		succeeded = false
	}

	err = k8sutil.DeleteDeployment(r.opManagerContext, r.context.Clientset, r.opConfig.OperatorNamespace, deployment)
	if err != nil {
		logger.Errorf("failed to delete the %q. %v", deployment, err)
		succeeded = false
	}

	err = k8sutil.DeleteCSIDriver(r.opManagerContext, r.context.Clientset, driverName)
	if err != nil {
		logger.Errorf("failed to delete the %q. %v", driverName, err)
		succeeded = false
	}
	return succeeded
}
