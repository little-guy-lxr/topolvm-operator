package csi

import (
	_ "embed"
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
	DefaultTopolvmImage = "build-harbor.alauda.cn/acp/topolvm:v3.6.0"
)

const (
	defaultLogLevel                   uint8 = 0
	provisionerTolerationsEnv               = "CSI_PROVISIONER_TOLERATIONS"
	provisionerNodeAffinityEnv              = "CSI_PROVISIONER_NODE_AFFINITY"
	pluginTolerationsEnv                    = "CSI_PROVISIONER_TOLERATIONS"
	pluginNodeAffinityEnv                   = "CSI_PROVISIONER_NODE_AFFINITY"
	topolvmProvisionerTolerationsEnv        = "TOPOLVM_PROVISIONER_TOLERATIONS"
	topolvmProvisionerNodeAffinityEnv       = "TOPOLVM_PROVISIONER_NODE_AFFINITY"
	TopolvmPluginTolerationsEnv             = "TOPOLVM_PLUGIN_TOLERATIONS"
	TopolvmPluginNodeAffinityEnv            = "TOPOLVM_PLUGIN_NODE_AFFINITY"

	topolvmProvisionerResource = "TOPOLVM_PROVISIONER_RESOURCE"
	TopolvmPluginResource      = "TOPOLVM_PLUGIN_RESOURCE"
	// default provisioner replicas
	defaultProvisionerReplicas int32 = 2

	csiTopolvmProvisioner = "topolvm-controller"
)

var (
	CSIParam csi.Param

	EnableTopolvm = false
	//driver names
	RawDeviceDriverName string
)

var (
	//go:embed template/csi-topolvm-plugin.yaml
	CSITopolvmPluginTemplatePath string
	//go:embed template/csi-topolvm-provisioner.yaml
	CSIToplvmProvisionerTemplatePath string
	//go:embed template/topolvm-csi-driver.yaml
	TopolvmCSIDriverTemplatePath string
)

func (r *CSITopolvmController) startDrivers(ver *version.Info, ownerInfo *k8sutil.OwnerInfo) error {
	logger.Info("start csi topolvm driver")
	var (
		err                error
		topolvmProvisioner *apps.Deployment
		topolvmPlugin      *apps.Deployment
		topolvmCSIDriver   *storagev1.CSIDriver
	)

	tp := csi.TemplateParam{
		Param:     CSIParam,
		Namespace: r.opConfig.OperatorNamespace,
	}

	// default value `system-node-critical` is the highest available priority
	tp.PluginPriorityClassName = k8sutil.GetValue(r.opConfig.Parameters, "CSI_PLUGIN_PRIORITY_CLASSNAME", "")

	// default value `system-cluster-critical` is applied for some
	// critical pods in cluster but less priority than plugin pods
	tp.ProvisionerPriorityClassName = k8sutil.GetValue(r.opConfig.Parameters, "CSI_PROVISIONER_PRIORITY_CLASSNAME", "")

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

	if EnableTopolvm {
		topolvmProvisioner, err = csi.TemplateToDeployment("topolvm-provisioner", CSIToplvmProvisionerTemplatePath, tp)
		if err != nil {
			return errors.Wrap(err, "failed to load topolvm provisioner deployment template")
		}

		topolvmPlugin, err = csi.TemplateToDeployment("topolvm-plugin", CSITopolvmPluginTemplatePath, tp)
		if err != nil {
			return errors.Wrap(err, "failed to load topolvm provisioner deployment template")
		}

		topolvmCSIDriver, err = csi.TemplateToCSIDriver("topolvm-csi-driver", TopolvmCSIDriverTemplatePath, tp)
		if err != nil {
			return errors.Wrap(err, "failed to load topolvm csi driver template")
		}
	}

	// get common provisioner tolerations and node affinity
	provisionerTolerations := csi.GetToleration(r.opConfig.Parameters, provisionerTolerationsEnv, []corev1.Toleration{})
	provisionerNodeAffinity := csi.GetNodeAffinity(r.opConfig.Parameters, provisionerNodeAffinityEnv, &corev1.NodeAffinity{})

	if topolvmProvisioner != nil {

		topolvmProvisionerTolerations := csi.GetToleration(r.opConfig.Parameters, topolvmProvisionerTolerationsEnv, provisionerTolerations)
		topolvmProvisionerNodeAffinity := csi.GetNodeAffinity(r.opConfig.Parameters, topolvmProvisionerNodeAffinityEnv, provisionerNodeAffinity)
		csi.ApplyToPodSpec(&topolvmProvisioner.Spec.Template.Spec, topolvmProvisionerNodeAffinity, topolvmProvisionerTolerations)
		// apply resource request and limit to rbd provisioner containers
		csi.ApplyResourcesToContainers(r.opConfig.Parameters, topolvmProvisionerResource, &topolvmProvisioner.Spec.Template.Spec)
		err = ownerInfo.SetControllerReference(topolvmProvisioner)
		if err != nil {
			return errors.Wrapf(err, "failed to set owner reference to topolvm provisioner deployment %q", topolvmProvisioner.Name)
		}
		antiAffinity := csi.GetPodAntiAffinity("app", csiTopolvmProvisioner)
		topolvmProvisioner.Spec.Template.Spec.Affinity.PodAntiAffinity = &antiAffinity
		topolvmProvisioner.Spec.Strategy = apps.DeploymentStrategy{
			Type: apps.RecreateDeploymentStrategyType,
		}
		_, err = k8sutil.CreateOrUpdateDeployment(r.opManagerContext, r.context.Clientset, topolvmProvisioner)
		if err != nil {
			return errors.Wrapf(err, "failed to start topolvm provisioner deployment %q", topolvmProvisioner.Name)
		}
		logger.Info("successfully started CSI topolvm driver")
	}
	if topolvmPlugin != nil {
		err = r.updateTopolvmPlugin(topolvmPlugin, ownerInfo)
		if err != nil {
			return errors.Wrapf(err, "failed to start topolvm plugin driver %q", topolvmPlugin.Name)
		}
	}
	if topolvmCSIDriver != nil {
		err = k8sutil.CreateOrUpdateCSIDriver(r.opManagerContext, r.context.Clientset, topolvmCSIDriver)
		if err != nil {
			return errors.Wrapf(err, "failed to start topolvm driver %q", topolvmCSIDriver.Name)
		}
	}

	return nil
}

func (r *CSITopolvmController) updateTopolvmPlugin(deployment *apps.Deployment, ownerInfo *k8sutil.OwnerInfo) error {

	pluginDeps, err := r.context.Clientset.AppsV1().Deployments(r.opConfig.OperatorNamespace).List(r.opManagerContext, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=topolvm-node",
	})
	if err != nil {
		return err
	}

	pluginTolerations := csi.GetToleration(r.opConfig.Parameters, pluginTolerationsEnv, []corev1.Toleration{})
	pluginNodeAffinity := csi.GetNodeAffinity(r.opConfig.Parameters, pluginNodeAffinityEnv, &corev1.NodeAffinity{})

	for _, dep := range pluginDeps.Items {
		plugin := deployment.DeepCopy()
		plugin.Spec.Template.Spec.NodeSelector = dep.Spec.Template.Spec.NodeSelector
		plugin.Name = dep.Name
		topolvmPluginTolerations := csi.GetToleration(r.opConfig.Parameters, TopolvmPluginTolerationsEnv, pluginTolerations)
		topolvmPluginNodeAffinity := csi.GetNodeAffinity(r.opConfig.Parameters, TopolvmPluginNodeAffinityEnv, pluginNodeAffinity)
		csi.ApplyToPodSpec(&plugin.Spec.Template.Spec, topolvmPluginNodeAffinity, topolvmPluginTolerations)
		csi.ApplyResourcesToContainers(r.opConfig.Parameters, TopolvmPluginResource, &plugin.Spec.Template.Spec)
		err = ownerInfo.SetControllerReference(plugin)
		if err != nil {
			return errors.Wrapf(err, "failed to set owner reference to topolvm plugin deployment %q", plugin.Name)
		}
		_, err = k8sutil.CreateOrUpdateDeployment(r.opManagerContext, r.context.Clientset, &dep)
		if err != nil {
			return errors.Wrapf(err, "failed to update topolvm provisioner deployment %q", plugin.Name)
		}
	}

	logger.Info("successfully upadted topolvm plugin")
	return nil

}

func (r *CSITopolvmController) stopDrivers(ver *version.Info) {
	if !EnableTopolvm {
		logger.Info("CSI topolvm driver disabled")
		succeeded := r.deleteCSIDriverResources(ver, csiTopolvmProvisioner, "topolvm.cybozu.com")
		if succeeded {
			logger.Info("successfully removed CSI topolvm driver")
		} else {
			logger.Error("failed to remove CSI topolvm driver")
		}
	}
}

func (r *CSITopolvmController) deleteCSIDriverResources(ver *version.Info, deployment, driverName string) bool {
	succeeded := true

	err := k8sutil.DeleteDeployment(r.opManagerContext, r.context.Clientset, r.opConfig.OperatorNamespace, deployment)
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
