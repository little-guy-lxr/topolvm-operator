/*
Copyright 2021 The Topolvm-Operator Authors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package volumectr

import (
	"context"
	"github.com/alauda/topolvm-operator/pkg/cluster/topolvm"

	"github.com/alauda/topolvm-operator/pkg/operator/k8sutil"
	"github.com/coreos/pkg/capnslog"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

var (
	logger = capnslog.NewPackageLogger("topolvm/operator", "controller-deployment")
)

func CreateReplaceTopolvmControllerDeployment(clientset kubernetes.Interface, ref *metav1.OwnerReference) error {

	deploymentName := topolvm.TopolvmControllerDeploymentName
	_, err := clientset.AppsV1().Deployments(topolvm.NameSpace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		logger.Errorf("failed to detect deployment:%s. err:%v", deploymentName, err)
		return err
	} else if err == nil {

		err := k8sutil.DeleteDeployment(clientset, topolvm.NameSpace, deploymentName)
		if err != nil {
			logger.Errorf("failed to remove deployment:%s. err:%v", deploymentName, err)
			return err
		}
	}
	return CreateControllerDeployment(clientset, ref)
}

func CreateControllerDeployment(clientset kubernetes.Interface, ref *metav1.OwnerReference) error {

	deployment, err := getDeployment(clientset, ref)
	if err != nil {
		logger.Errorf("create node deployment %s failed err %s", topolvm.TopolvmControllerDeploymentName, err)
		return err
	}

	if err := k8sutil.CreateDeployment(clientset, topolvm.TopolvmControllerDeploymentName, topolvm.NameSpace, deployment); err != nil {
		logger.Errorf("create node deployment %s failed err %s", topolvm.TopolvmControllerDeploymentName, err)
		return err
	}
	return nil
}

func getDeployment(clientset kubernetes.Interface, ref *metav1.OwnerReference) (*v1.Deployment, error) {

	replicas := int32(1)

	if nodes, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{Limit: 2}); err == nil {
		if len(nodes.Items) > 1 {
			replicas = int32(2)
		}
	} else {
		logger.Errorf("unable to list nodes and deploying single csi controller")
	}

	volumes := []corev1.Volume{
		{Name: "socket-dir", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
	}
	iContainers := []corev1.Container{}

	certsFound := false
	if topolvm.CertsSecret != "" {
		if _, err := clientset.CoreV1().Secrets(topolvm.NameSpace).Get(context.TODO(), topolvm.CertsSecret, metav1.GetOptions{}); err != nil {
			if errors.IsNotFound(err) {
				logger.Errorf("Secret %s provided in cluster CRD not found", topolvm.CertsSecret)
			} else {
				logger.Errorf("Unexpected error trying to locate secret %q: %v", topolvm.CertsSecret, err)
			}
			return nil, err
		} else {
			certsFound = true
		}
	}

	if certsFound {
		volumes = append(volumes, corev1.Volume{Name: "certs", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: topolvm.CertsSecret}}})
		logger.Infof("topolvm-controller will be deployed using the certificate provided in secret %s", topolvm.CertsSecret)
	} else {
		volumes = append(volumes, corev1.Volume{Name: "certs", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}})
		iContainers = append(iContainers, *getInitContainer())
		logger.Info("topolvm-controller will be deployed using an automatically generated self signed certificate")
	}

	containers := []corev1.Container{*getControllerContainer(), *getCsiProvisionerContainer(), *getCsiResizerContainer(), *getLivenessProbeContainer()}

	var maxSurge, maxUnavailable intstr.IntOrString
	maxSurge.IntVal = 1
	maxUnavailable.IntVal = 1
	controllerDeployment := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            topolvm.TopolvmControllerDeploymentName,
			Namespace:       topolvm.NameSpace,
			OwnerReferences: []metav1.OwnerReference{*ref},
		},
		Spec: v1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					topolvm.AppAttr: topolvm.TopolvmControllerDeploymentName,
				},
			},
			Strategy: v1.DeploymentStrategy{
				RollingUpdate: &v1.RollingUpdateDeployment{
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      topolvm.TopolvmControllerDeploymentName,
					Namespace: topolvm.NameSpace,
					Labels: map[string]string{
						topolvm.AppAttr: topolvm.TopolvmControllerDeploymentName,
					},
				},
				Spec: corev1.PodSpec{
					InitContainers:     iContainers,
					Containers:         containers,
					ServiceAccountName: topolvm.ContollerServiceAccount,
					Volumes:            volumes,
					Tolerations:        []corev1.Toleration{{Operator: corev1.TolerationOpExists}},
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{Key: topolvm.AppAttr, Operator: metav1.LabelSelectorOpIn, Values: []string{topolvm.TopolvmControllerDeploymentName}}},
									},
									TopologyKey: topolvm.TopologKey,
								},
							},
						},
					},
				},
			},
		},
	}
	return controllerDeployment, nil

}
func getInitContainer() *corev1.Container {
	command := []string{
		"sh",
		"-c",
		"openssl req -nodes -x509 -newkey rsa:4096 -subj '/DC=self_signed_certificate' -keyout /certs/tls.key -out /certs/tls.crt -days 365",
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "certs", MountPath: "/certs"},
	}

	ssCertGenerator := &corev1.Container{
		Name:         "self-signed-cert-generator",
		Image:        "alpine/openssl",
		Command:      command,
		VolumeMounts: volumeMounts,
	}

	return ssCertGenerator
}

func getControllerContainer() *corev1.Container {

	command := []string{
		"/topolvm-controller",
		"--cert-dir=/certs",
	}

	resourceRequirements := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(topolvm.TopolvmControllerCPULimit),
			corev1.ResourceMemory: resource.MustParse(topolvm.TopolvmControllerMemLimit),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(topolvm.TopolvmControllerCPURequest),
			corev1.ResourceMemory: resource.MustParse(topolvm.TopolvmControllerMemRequest),
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "socket-dir", MountPath: "/run/topolvm"},
		{Name: "certs", MountPath: "/certs"},
	}

	controller := &corev1.Container{
		Name:    topolvm.TopolvmControllerContainerName,
		Image:   topolvm.TopolvmImage,
		Command: command,
		Ports:   []corev1.ContainerPort{{Name: topolvm.TopolvmControllerContainerHealthzName, ContainerPort: topolvm.TopolvmControllerContainerLivenessPort, Protocol: corev1.ProtocolTCP}},
		LivenessProbe: &corev1.Probe{Handler: corev1.Handler{HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromString(topolvm.TopolvmControllerContainerHealthzName)}},
			FailureThreshold: 3, InitialDelaySeconds: 10, TimeoutSeconds: 3, PeriodSeconds: 60},
		ReadinessProbe: &corev1.Probe{Handler: corev1.Handler{HTTPGet: &corev1.HTTPGetAction{Path: "/metrics", Port: intstr.IntOrString{IntVal: topolvm.TopolvmControllerContainerReadinessPort}, Scheme: corev1.URISchemeHTTP}}},
		Resources:      resourceRequirements,
		VolumeMounts:   volumeMounts,
	}
	return controller
}

func getCsiResizerContainer() *corev1.Container {

	command := []string{
		"/csi-resizer",
		"--csi-address=/run/topolvm/csi-topolvm.sock",
		"--leader-election",
		"--leader-election-namespace=" + topolvm.NameSpace,
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "socket-dir", MountPath: "/run/topolvm"},
	}

	csiResizer := &corev1.Container{
		Name:         topolvm.TopolvmCsiResizerContainerName,
		Image:        topolvm.TopolvmImage,
		Command:      command,
		VolumeMounts: volumeMounts,
	}
	return csiResizer
}
func getCsiProvisionerContainer() *corev1.Container {

	command := []string{"/csi-provisioner",
		"--csi-address=/run/topolvm/csi-topolvm.sock",
		"--enable-capacity",
		"--capacity-ownerref-level=2",
		"--capacity-poll-interval=30s",
		"--feature-gates=Topology=true",
		"--leader-election",
		"--leader-election-namespace=" + topolvm.NameSpace,
	}

	resourceRequirements := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(topolvm.TopolvmControllerCsiProvisionCPULimit),
			corev1.ResourceMemory: resource.MustParse(topolvm.TopolvmControllerCsiProvisionMemLimit),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(topolvm.TopolvmControllerCsiProvisionCPURequest),
			corev1.ResourceMemory: resource.MustParse(topolvm.TopolvmControllerCsiProvisionMemRequest),
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "socket-dir", MountPath: "/run/topolvm"},
	}

	env := []corev1.EnvVar{
		{Name: topolvm.PodNameSpaceEnv, ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
		{Name: topolvm.PodNameEnv, ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
		{Name: topolvm.NameSpaceEnv, ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
	}

	csiProvisioner := &corev1.Container{
		Name:         topolvm.TopolvmCsiProvisionerContainerName,
		Image:        topolvm.TopolvmImage,
		Command:      command,
		Resources:    resourceRequirements,
		VolumeMounts: volumeMounts,
		Env:          env,
	}
	return csiProvisioner
}

func getLivenessProbeContainer() *corev1.Container {

	command := []string{
		"/livenessprobe",
		"--csi-address=/run/topolvm/csi-topolvm.sock",
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "socket-dir", MountPath: "/run/topolvm"},
	}

	livenessProbe := &corev1.Container{
		Name:         topolvm.TopolvmCsiLivenessProbeContainerName,
		Image:        topolvm.TopolvmImage,
		Command:      command,
		VolumeMounts: volumeMounts,
	}
	return livenessProbe
}
