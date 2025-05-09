/*
Copyright 2025 Chia Network Inc.
*/

package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

func TestUnmarshalChiaDataLayer(t *testing.T) {
	yamlData := []byte(`
apiVersion: k8s.chia.net/v1
kind: ChiaDataLayer
metadata:
  labels:
    app.kubernetes.io/name: chiadatalayer
    app.kubernetes.io/instance: chiadatalayer-sample
    app.kubernetes.io/part-of: chia-operator
    app.kubernetes.io/created-by: chia-operator
  name: chiadatalayer-sample
spec:
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 1
  chia:
    caSecretName: chiaca-secret
    testnet: true
    network: testnet68419
    networkPort: 8080
    introducerAddress: introducer.svc.cluster.local
    dnsIntroducerAddress: dns-introducer.svc.cluster.local
    timezone: "UTC"
    logLevel: "INFO"
    fullNodePeers: 
      - host: "node.default.svc.cluster.local"
        port: 58444
    secretKey:
      name: "chiakey-secret"
      key: "key.txt"
    trustedCIDRs:
      - "192.168.0.0/16"
      - "10.0.0.0/8"
    xchSpamAmount: 0
  fileserver:
    enabled: true
    service:
      enabled: true
      labels:
        key: value
    ingress:
      enabled: true
      ingressClassName: nginx
      host: datalayer.example.com
      tls:
        - hosts:
            - datalayer.example.com
          secretName: datalayer-tls
      rules:
        - host: datalayer.example.com
          http:
            paths:
              - path: /
                pathType: Prefix
                backend:
                  service:
                    name: chiadatalayer-sample-fileserver
                    port:
                      number: 8575
  chiaExporter:
    enabled: true
    service:
    serviceLabels:
      network: testnet
`)

	var (
		testTrue                    = true
		timezone                    = "UTC"
		logLevel                    = "INFO"
		network                     = "testnet68419"
		networkPort          uint16 = 8080
		introducerAddress           = "introducer.svc.cluster.local"
		dnsIntroducerAddress        = "dns-introducer.svc.cluster.local"
		caSecret                    = "chiaca-secret"
		ingressClassName            = "nginx"
		host                        = "datalayer.example.com"
		strategy                    = appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
			RollingUpdate: &appsv1.RollingUpdateDeployment{
				MaxSurge:       &intstr.IntOrString{IntVal: 1},
				MaxUnavailable: &intstr.IntOrString{IntVal: 1},
			},
		}
		xchSpamAmount uint64 = 0
	)
	expectCIDRs := []string{
		"192.168.0.0/16",
		"10.0.0.0/8",
	}
	expectTLS := []networkingv1.IngressTLS{
		{
			Hosts:      []string{"datalayer.example.com"},
			SecretName: "datalayer-tls",
		},
	}
	expectRules := []networkingv1.IngressRule{
		{
			Host: "datalayer.example.com",
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     "/",
							PathType: ptr.To(networkingv1.PathTypePrefix),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: "chiadatalayer-sample-fileserver",
									Port: networkingv1.ServiceBackendPort{
										Number: 8575,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	expect := ChiaDataLayer{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "k8s.chia.net/v1",
			Kind:       "ChiaDataLayer",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "chiadatalayer-sample",
			Labels: map[string]string{
				"app.kubernetes.io/name":       "chiadatalayer",
				"app.kubernetes.io/instance":   "chiadatalayer-sample",
				"app.kubernetes.io/part-of":    "chia-operator",
				"app.kubernetes.io/created-by": "chia-operator",
			},
		},
		Spec: ChiaDataLayerSpec{
			Strategy: &strategy,
			ChiaConfig: ChiaDataLayerSpecChia{
				CommonSpecChia: CommonSpecChia{
					Testnet:              &testTrue,
					Timezone:             &timezone,
					LogLevel:             &logLevel,
					Network:              &network,
					NetworkPort:          &networkPort,
					IntroducerAddress:    &introducerAddress,
					DNSIntroducerAddress: &dnsIntroducerAddress,
				},
				CASecretName: &caSecret,
				FullNodePeers: &[]Peer{
					{
						Host: "node.default.svc.cluster.local",
						Port: 58444,
					},
				},
				SecretKey: ChiaSecretKey{
					Name: "chiakey-secret",
					Key:  "key.txt",
				},
				TrustedCIDRs:  &expectCIDRs,
				XCHSpamAmount: &xchSpamAmount,
			},
			CommonSpec: CommonSpec{
				ChiaExporterConfig: SpecChiaExporter{
					Enabled: boolPtr(true),
				},
			},
			FileserverConfig: FileserverConfig{
				Enabled: &testTrue,
				Service: Service{
					Enabled: &testTrue,
					AdditionalMetadata: AdditionalMetadata{
						Labels: map[string]string{
							"key": "value",
						},
					},
				},
				Ingress: IngressConfig{
					Enabled:          &testTrue,
					IngressClassName: &ingressClassName,
					Host:             &host,
					TLS:              &expectTLS,
					Rules:            &expectRules,
				},
			},
		},
	}

	var actual ChiaDataLayer
	err := yaml.Unmarshal(yamlData, &actual)
	if err != nil {
		t.Errorf("Error unmarshaling yaml: %v", err)
		return
	}

	diff := cmp.Diff(actual, expect)
	if diff != "" {
		t.Errorf("Unmarshaled struct does not match the expected struct. Actual: %+v\nExpected: %+v\nDiff: %s", actual, expect, diff)
		return
	}
}
