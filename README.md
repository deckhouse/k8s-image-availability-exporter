# k8s-image-availability-exporter

[![Go Reference](https://pkg.go.dev/badge/github.com/flant/k8s-image-availability-exporter.svg)](https://pkg.go.dev/github.com/flant/k8s-image-availability-exporter)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/k8s-image-availability-exporter)](https://artifacthub.io/packages/search?repo=k8s-image-availability-exporter)

k8s-image-availability-exporter (or *k8s-iae* for short) is a Prometheus exporter that warns you proactively about images that are defined in Kubernetes objects (e.g., an `image` field in the Deployment) but are not available in the container registry (such as Docker Registry, etc.).

Receiving alerts when container images related to running Kubernetes controllers are missing helps you to solve the problem before it manifests itself. For more details on the reasons behind k8s-iae and how it works, please read [this article](https://medium.com/flant-com/prometheus-exporter-to-check-kubernetes-images-availability-26c306c44c08).

* [Deploying / Installing](#deploying) k8s-iae in your Kubernetes cluster
  * [Prometheus integration](#prometheus-integration) to scrape metrics
  * [Alerting](#alerting) based on k8s-iae metrics
* [Configuration](#configuration)
  * [CLI options](#command-line-options)
* [Metrics](#metrics) for Prometheus provided by k8s-iae
* [Compatibility](#compatibility)

## Deploying

### Container image

Ready-to-use container images are available in the Deckhouse registry:

```bash
docker pull registry.deckhouse.io/k8s-image-availability-exporter/k8s-image-availability-exporter:latest
```

### Helm Chart

The helm chart is available on [artifacthub](https://artifacthub.io/packages/helm/k8s-image-availability-exporter/k8s-image-availability-exporter). Follow instructions on the page to install it.

### Prometheus integration
 
Here's how you can configure Prometheus or prometheus-operator to scrape metrics from `k8s-image-availability-exporter`.
 
#### Prometheus

```yaml
- job_name: image-availability-exporter
  honor_labels: true
  metrics_path: '/metrics'
  scheme: http
  kubernetes_sd_configs:
  - role: pod
    namespaces:
      names:
      - kube-system
  relabel_configs:
  - source_labels: [__meta_kubernetes_pod_label_app]
    regex: image-availability-exporter
    action: keep
```

#### prometheus-operator

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: image-availability-exporter
  namespace: kube-system
spec:
  podMetricsEndpoints:
  - port: http-metrics
    scheme: http
    honorLabels: true
    scrapeTimeout: 10s
  selector:
    matchLabels:
      app: image-availability-exporter
  namespaceSelector:
    matchNames:
    - kube-system
```

### Alerting

Here's how to alert based on these metrics:

#### Prometheus

```yaml
groups:
- name: image-availability-exporter.rules
  rules:
  - alert: DeploymentImageUnavailable
    expr: |
      max by (namespace, deployment, container, image) (
        k8s_image_availability_exporter_deployment_available == 0
      )
    annotations:
      description: >
        Check image's `{{ $labels.image }}` availability in container registry
        in Namespace `{{ $labels.namespace }}`
        in Deployment `{{ $labels.owner_name }}`
        in container `{{ $labels.container }}` in registry.
      summary: Image `{{ $labels.image }}` is unavailable in container registry.

  - alert: StatefulSetImageUnavailable
    expr: |
      max by (namespace, statefulset, container, image) (
        k8s_image_availability_exporter_statefulset_available == 0
      )
    annotations:
      description: >
        Check image's `{{ $labels.image }}` availability in container registry
        in Namespace `{{ $labels.namespace }}`
        in StatefulSet `{{ $labels.owner_name }}`
        in container `{{ $labels.container }}` in registry.
      summary: Image `{{ $labels.image }}` is unavailable in container registry.

  - alert: DaemonSetImageUnavailable
    expr: |
      max by (namespace, daemonset, container, image) (
        k8s_image_availability_exporter_daemonset_available == 0
      )
    annotations:
      description: >
        Check image's `{{ $labels.image }}` availability in container registry
        in Namespace `{{ $labels.namespace }}`
        in DaemonSet `{{ $labels.owner_name }}`
        in container `{{ $labels.container }}` in registry.
      summary: Image `{{ $labels.image }}` is unavailable in container registry.

  - alert: CronJobImageUnavailable
    expr: |
      max by (namespace, cronjob, container, image) (
        k8s_image_availability_exporter_cronjob_available == 0
      )
    annotations:
      description: >
        Check image's `{{ $labels.image }}` availability in container registry
        in Namespace `{{ $labels.namespace }}`
        in CronJob `{{ $labels.owner_name }}`
        in container `{{ $labels.container }}` in registry.
      summary: Image `{{ $labels.image }}` is unavailable in container registry.
```

#### prometheus-operator

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: image-availability-exporter-alerts
  namespace: kube-system
spec:
  groups:
  - name: image-availability-exporter.rules
    rules:

    - alert: DeploymentImageUnavailable
      expr: |
        max by (namespace, deployment, container, image) (
          k8s_image_availability_exporter_deployment_available == 0
        )
      annotations:
        description: >
          Check image's `{{ $labels.image }}` availability in container registry
          in Namespace `{{ $labels.namespace }}`
          in Deployment `{{ $labels.owner_name }}`
          in container `{{ $labels.container }}` in registry.
        summary: Image `{{ $labels.image }}` is unavailable.
    
    - alert: StatefulSetImageUnavailable
      expr: |
        max by (namespace, statefulset, container, image) (
          k8s_image_availability_exporter_statefulset_available == 0
        )
      annotations:
        description: >
          Check image's `{{ $labels.image }}` availability in container registry
          in Namespace `{{ $labels.namespace }}`
          in StatefulSet `{{ $labels.owner_name }}`
          in container `{{ $labels.container }}` in registry.
        summary: Image `{{ $labels.image }}` is unavailable in container registry.
    
    - alert: DaemonSetImageUnavailable
      expr: |
        max by (namespace, daemonset, container, image) (
          k8s_image_availability_exporter_daemonset_available == 0
        )
      annotations:
        description: >
          Check image's `{{ $labels.image }}` availability in container registry
          in Namespace `{{ $labels.namespace }}`
          in DaemonSet `{{ $labels.owner_name }}`
          in container `{{ $labels.container }}` in registry.
        summary: Image `{{ $labels.image }}` is unavailable in container registry.
    
    - alert: CronJobImageUnavailable
      expr: |
        max by (namespace, cronjob, container, image) (
          k8s_image_availability_exporter_cronjob_available == 0
        )
      annotations:
        description: >
          Check image's `{{ $labels.image }}` availability in container registry
          in Namespace `{{ $labels.namespace }}`
          in CronJob `{{ $labels.owner_name }}`
          in container `{{ $labels.container }}` in registry.
        summary: Image `{{ $labels.image }}` is unavailable in container registry.
```

## Configuration

### Command-line options

```
Usage of k8s-image-availability-exporter:
  -bind-address string
        address:port to bind /metrics endpoint to (default ":8080")
  -check-interval duration
        image re-check interval (default 1m0s)
  -default-registry string
        default registry to use in absence of a fully qualified image name, defaults to "index.docker.io"
  -ignored-images string
        tilde-separated image regexes to ignore, each image will be checked against this list of regexes
  -namespace-label string
        namespace label for checks
  -skip-registry-cert-verification
        whether to skip registries' certificate verification
```

## Metrics

The following metrics for Prometheus are provided:

* `k8s_image_availability_exporter_<TYPE>_available` — non-zero indicates *successful* image check.
* `k8s_image_availability_exporter_<TYPE>_bad_image_format` — non-zero indicates incorrect `image` field format.
* `k8s_image_availability_exporter_<TYPE>_absent` — non-zero indicates an image's manifest absence from container registry.
* `k8s_image_availability_exporter_<TYPE>_registry_unavailable` — non-zero indicates general registry unavailiability, perhaps, due to network outage.
* `k8s_image_availability_exporter_deployment_registry_v1_api_not_supported` — non-zero indicates v1 Docker Registry API, these images are best ignored with `--ignored-images` cmdline parameter.
* `k8s_image_availability_exporter_<TYPE>_authentication_failure` — non-zero indicates authentication error to container registry, verify imagePullSecrets.
* `k8s_image_availability_exporter_<TYPE>_authorization_failure` — non-zero indicates authorization error to container registry, verify imagePullSecrets.
* `k8s_image_availability_exporter_<TYPE>_unknown_error` — non-zero indicates an error that failed to be classified, consult exporter's logs for additional information.

Each `<TYPE>` in the exporter's metrics name is replaced with the following values:

* `deployment`
* `statefulset`
* `daemonset` 
* `cronjob`

## Compatibility

k8s-image-availability-exporter is compatible with Kubernetes 1.15+ and Docker Registry V2 compliant container registries.

Since the exporter operates as a Deployment, it *does not* support container registries that should be accessed via authorization on a node.
