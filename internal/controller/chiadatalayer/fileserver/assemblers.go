package fileserver

import (
	"fmt"

	k8schianetv1 "github.com/chia-network/chia-operator/api/v1"
	"github.com/chia-network/chia-operator/internal/controller/common/consts"
	"github.com/chia-network/chia-operator/internal/controller/common/kube"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const chiadatalayerfileserverNamePattern = "%s-datalayer-http"

// AssembleService assembles the fileserver Service resource for a ChiaDataLayer CR
func AssembleService(datalayer k8schianetv1.ChiaDataLayer) corev1.Service {
	inputs := kube.AssembleCommonServiceInputs{
		Name:      fmt.Sprintf(chiadatalayerfileserverNamePattern, datalayer.Name),
		Namespace: datalayer.Namespace,
		Ports: []corev1.ServicePort{
			{
				Port:       80,
				TargetPort: intstr.FromString("http"),
				Protocol:   "TCP",
				Name:       "http",
			},
		},
	}

	inputs.ServiceType = datalayer.Spec.FileserverConfig.Service.ServiceType
	inputs.ExternalTrafficPolicy = datalayer.Spec.FileserverConfig.Service.ExternalTrafficPolicy
	inputs.SessionAffinity = datalayer.Spec.FileserverConfig.Service.SessionAffinity
	inputs.SessionAffinityConfig = datalayer.Spec.FileserverConfig.Service.SessionAffinityConfig
	inputs.IPFamilyPolicy = datalayer.Spec.FileserverConfig.Service.IPFamilyPolicy
	inputs.IPFamilies = datalayer.Spec.FileserverConfig.Service.IPFamilies

	// Labels
	var additionalServiceLabels = make(map[string]string)
	if datalayer.Spec.FileserverConfig.Service.Labels != nil {
		additionalServiceLabels = datalayer.Spec.FileserverConfig.Service.Labels
	}
	inputs.Labels = kube.GetCommonLabels(datalayer.Kind, datalayer.ObjectMeta, datalayer.Spec.Labels, additionalServiceLabels)
	inputs.SelectorLabels = kube.GetCommonLabels(datalayer.Kind, datalayer.ObjectMeta, datalayer.Spec.Labels)

	// Annotations
	var additionalServiceAnnotations = make(map[string]string)
	if datalayer.Spec.FileserverConfig.Service.Annotations != nil {
		additionalServiceAnnotations = datalayer.Spec.FileserverConfig.Service.Annotations
	}
	inputs.Annotations = kube.CombineMaps(datalayer.Spec.Annotations, additionalServiceAnnotations)

	return kube.AssembleCommonService(inputs)
}

// AssembleContainer creates and configures a Kubernetes container for the fileserver based on the provided spec.
func AssembleContainer(datalayer k8schianetv1.ChiaDataLayer) corev1.Container {
	container := corev1.Container{
		Name:            "fileserver",
		Image:           fmt.Sprintf("%s:%s", consts.DefaultChiaImageName, consts.DefaultChiaImageTag),
		ImagePullPolicy: datalayer.Spec.ImagePullPolicy,
	}

	if datalayer.Spec.FileserverConfig.LivenessProbe != nil {
		container.LivenessProbe = datalayer.Spec.FileserverConfig.LivenessProbe
	}

	if datalayer.Spec.FileserverConfig.ReadinessProbe != nil {
		container.ReadinessProbe = datalayer.Spec.FileserverConfig.ReadinessProbe
	}

	if datalayer.Spec.FileserverConfig.StartupProbe != nil {
		container.StartupProbe = datalayer.Spec.FileserverConfig.StartupProbe
	}

	if datalayer.Spec.FileserverConfig.Resources != nil {
		container.Resources = *datalayer.Spec.FileserverConfig.Resources
	}

	if datalayer.Spec.FileserverConfig.SecurityContext != nil {
		container.SecurityContext = datalayer.Spec.FileserverConfig.SecurityContext
	}

	// Set image
	usingCustomImage := false
	if datalayer.Spec.FileserverConfig.Image != nil && *datalayer.Spec.FileserverConfig.Image != "" {
		container.Image = *datalayer.Spec.FileserverConfig.Image
		usingCustomImage = true
	}

	// Set http container port
	containerPort := consts.DataLayerHTTPPort
	if datalayer.Spec.FileserverConfig.ContainerPort != nil {
		containerPort = *datalayer.Spec.FileserverConfig.ContainerPort
	}
	container.Ports = []corev1.ContainerPort{
		{
			Name:          "http",
			ContainerPort: int32(containerPort),
			Protocol:      "TCP",
		},
	}

	// Set volume mountpath
	mountPath := "/datalayer/server"
	if datalayer.Spec.FileserverConfig.ServerFileMountpath != nil && *datalayer.Spec.FileserverConfig.ServerFileMountpath != "" {
		mountPath = *datalayer.Spec.FileserverConfig.ServerFileMountpath
	}
	container.VolumeMounts = []corev1.VolumeMount{
		{
			Name:      "server",
			MountPath: mountPath,
		},
	}

	// Set container env
	if usingCustomImage {
		// Using custom image
		if datalayer.Spec.FileserverConfig.AdditionalEnv != nil {
			container.Env = append(container.Env, *datalayer.Spec.FileserverConfig.AdditionalEnv...)
		}
	} else {
		// Using default chia image
		container.Env = []corev1.EnvVar{
			{
				Name:  "service",
				Value: "data_layer_http",
			},
			{
				Name:  "keys",
				Value: "none",
			},
			{
				Name:  "chia.data_layer.server_files_location",
				Value: mountPath,
			},
			{
				Name:  "chia.daemon_port",
				Value: "55401", // Avoids port conflict with the main chia container
			},
		}
		if datalayer.Spec.FileserverConfig.AdditionalEnv != nil {
			container.Env = append(container.Env, *datalayer.Spec.FileserverConfig.AdditionalEnv...)
		}
	}

	return container
}

// AssembleIngress assembles the fileserver Ingress resource for a ChiaDataLayer CR
func AssembleIngress(datalayer k8schianetv1.ChiaDataLayer) networkingv1.Ingress {
	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(chiadatalayerfileserverNamePattern, datalayer.Name),
			Namespace: datalayer.Namespace,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: datalayer.Spec.FileserverConfig.Ingress.IngressClassName,
		},
	}

	// Set labels
	var additionalIngressLabels = make(map[string]string)
	if datalayer.Spec.FileserverConfig.Ingress.Labels != nil {
		additionalIngressLabels = datalayer.Spec.FileserverConfig.Ingress.Labels
	}
	ingress.Labels = kube.GetCommonLabels(datalayer.Kind, datalayer.ObjectMeta, datalayer.Spec.Labels, additionalIngressLabels)

	// Set annotations
	var additionalIngressAnnotations = make(map[string]string)
	if datalayer.Spec.FileserverConfig.Ingress.Annotations != nil {
		additionalIngressAnnotations = datalayer.Spec.FileserverConfig.Ingress.Annotations
	}
	ingress.Annotations = kube.CombineMaps(datalayer.Spec.Annotations, additionalIngressAnnotations)

	// Set TLS if configured
	if datalayer.Spec.FileserverConfig.Ingress.TLS != nil {
		ingress.Spec.TLS = *datalayer.Spec.FileserverConfig.Ingress.TLS
	}

	// Set rules if configured
	if datalayer.Spec.FileserverConfig.Ingress.Rules != nil {
		ingress.Spec.Rules = *datalayer.Spec.FileserverConfig.Ingress.Rules
	} else if datalayer.Spec.FileserverConfig.Ingress.Host != nil {
		// Default rule if host is specified but no rules
		ingress.Spec.Rules = []networkingv1.IngressRule{
			{
				Host: *datalayer.Spec.FileserverConfig.Ingress.Host,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path:     "/",
								PathType: ptr.To(networkingv1.PathTypePrefix),
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: fmt.Sprintf(chiadatalayerfileserverNamePattern, datalayer.Name),
										Port: networkingv1.ServiceBackendPort{
											Number: 80,
										},
									},
								},
							},
						},
					},
				},
			},
		}
	}

	return ingress
}
