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
  - alert: DeploymentImageUnavailable
    expr: |
      max by (namespace, name, container, image) (
        k8s_image_availability_exporter_available{kind="deployment"} == 0
      )
    annotations:
      message: >
        Image {{`{{ $labels.image }}`}} from container {{`{{ $labels.container }}`}}
        in deployment {{`{{ $labels.name }}`}}
        from namespace {{`{{ $labels.namespace }}`}}
        is not available in docker registry.
    labels:
      severity: critical
  - alert: StatefulSetImageUnavailable
    expr: |
      max by (namespace, name, container, image) (
        k8s_image_availability_exporter_available{kind="statefulset"} == 0
      )
    annotations:
      message: >
        Image {{`{{ $labels.image }}`}} from container {{`{{ $labels.container }}`}}
        in statefulSet {{`{{ $labels.name }}`}}
        from namespace {{`{{ $labels.namespace }}`}}
        is not available in docker registry.
    labels:
      severity: critical
  - alert: DaemonSetImageUnavailable
    expr: |
      max by (namespace, name, container, image) (
        k8s_image_availability_exporter_available{kind="daemonset"} == 0
      )
    annotations:
      message: >
        Image {{`{{ $labels.image }}`}} from container {{`{{ $labels.container }}`}}
        in daemonSet {{`{{ $labels.name }}`}}
        from namespace {{`{{ $labels.namespace }}`}}
        is not available in docker registry.
    labels:
      severity: critical
  - alert: CronJobImageUnavailable
    expr: |
      max by (namespace, name, container, image) (
        k8s_image_availability_exporter_available{kind="cronjob"} == 0
      )
    annotations:
      message: >
        Image {{`{{ $labels.image }}`}} from container {{`{{ $labels.container }}`}}
        in cronJob {{`{{ $labels.name }}`}}
        from namespace {{`{{ $labels.namespace }}`}}
        is not available in docker registry.
    labels:
      severity: critical
```

## Configuration

### Command-line options

```
Usage of k8s-image-availability-exporter:
  -allow-plain-http
    	whether to fallback to HTTP scheme for registries that don't support HTTPS
  -bind-address string
    	address:port to bind /metrics endpoint to (default ":8080")
  -capath value
    	path to a file that contains CA certificates in the PEM format
  -check-interval duration
    	image re-check interval (default 1m0s)
  -default-registry string
    	default registry to use in absence of a fully qualified image name, defaults to "index.docker.io"
  -force-check-disabled-controllers value
    	comma-separated list of controller kinds for which image is forcibly checked, even when workloads are disabled or suspended. Acceptable values include "Deployment", "StatefulSet", "DaemonSet", "Cronjob" or "*" for all kinds (this option is case-insensitive)
  -ignored-images string
    	tilde-separated image regexes to ignore, each image will be checked against this list of regexes
  -image-mirror value
    	Add a mirror repository (format: original=mirror)
  -namespace-label string
    	namespace label for checks
  -skip-registry-cert-verification
    	whether to skip registries' certificate verification
  -ecr-images-exists
      whether images from ECR in your cluster
```

## Metrics

The following metrics for Prometheus are provided:

* `k8s_image_availability_exporter_available` — non-zero indicates *successful* image check.
* `k8s_image_availability_exporter_absent` — non-zero indicates an image's manifest absence from container registry.
* `k8s_image_availability_exporter_bad_image_format` — non-zero indicates incorrect `image` field format.
* `k8s_image_availability_exporter_registry_unavailable` — non-zero indicates general registry unavailiability, perhaps, due to network outage.
* `k8s_image_availability_exporter_authentication_failure` — non-zero indicates authentication error to container registry, verify imagePullSecrets.
* `k8s_image_availability_exporter_authorization_failure` — non-zero indicates authorization error to container registry, verify imagePullSecrets.
* `k8s_image_availability_exporter_unknown_error` — non-zero indicates an error that failed to be classified, consult exporter's logs for additional information.

Each metric has the following labels:

* `namespace` - namespace name
* `container` - container name
* `image` - image URL in the registry
* `kind` - Kubernetes controller kind, namely `deployment`, `statefulset`, `daemonset` or `cronjob`
* `name` - controller name

## Compatibility

k8s-image-availability-exporter is compatible with Kubernetes 1.15+ and Docker Registry V2 compliant container registries.

Since the exporter operates as a Deployment, it *does not* support container registries that should be accessed via authorization on a node.
