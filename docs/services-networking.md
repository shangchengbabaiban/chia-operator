# Services and Networking

Note: The following documentation applies to all Chia Operator resources except for ChiaCAs which don't require networking (besides Egress to the kubernetes API to make the certificate authority Secret.)

## Configuring Services

Multiple Services may be made for each Chia resource. One for the Chia daemon, one for the RPC API, and one for peer connections. ChiaNodes actually make two more peer Service variants: one headless Service, and one Local traffic policy Service.

An additional Service may be configured for the optional chia-exporter sidecar container for Chia metrics.

Below is an example ChiaNode that configures each of these Services, but the same applies to the other Chia resources:

```yaml
apiVersion: k8s.chia.net/v1
kind: ChiaNode
metadata:
  name: mainnet-node
spec:
  chia:
    caSecretName: chiaca-secret
    allService:
      enabled: true
      type: ClusterIP    
    peerService:
      enabled: true
      type: LoadBalancer
    daemonService: # will be ClusterIP by default
      enabled: false
    rpcService:
      enabled: true
      type: NodePort
  chiaExporter:
    service:
      enabled: true
      type: ClusterIP
```

You can enable or disable each Service individually, or change their Service type.

## Add Labels/Annotations

You may want to add some labels to your Services. Shown below is the peer Service configuration, but the same applies to all Service configuration sections.

```yaml
spec:
  chia:
    peerService:
      labels:
        network: mainnet
        component: full_node
```

You can do the same thing with annotations.

```yaml
spec:
  chia:
    peerService:
      annotations:
        hello: world
```

## Dual-stack Services

You may want to configure the IP families of a Service to have the operator generate IPv4 and/or IPv6 services. See the [kube documentation](https://kubernetes.io/docs/concepts/services-networking/dual-stack/#services) on dual stack Services for generic usage information.

Here's an example peer service configuration that will result in a dual stack Service:

```yaml
spec:
  chia:
    peerService:
      ipFamilyPolicy: PreferDualStack
      ipFamilies:
        - IPv4
        - IPv6
```

## External Traffic Policy

Depending on the configuration of your cluster, it may be useful to change the external traffic policy for any publicly exposed services.

```yaml
spec:
  chia:
    peerService:
      enabled: true
      type: LoadBalancer
      externalTrafficPolicy: Local
```

## Session Affinity

Route internal traffic to a consistent backend by setting `sessionAffinity` to `ClientIP`.

```yaml
spec:
  chia:
    rpcService:
      sessionAffinity: ClientIP
```

You can further control the session affinity settings if the default values do not work for your needs.

```yaml
spec:
  chia:
    rpcService:
      sessionAffinity: ClientIP
      sessionAffinityConfig:
        clientIP:
          timeoutSeconds: 300
```
