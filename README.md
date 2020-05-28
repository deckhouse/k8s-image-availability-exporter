# k8s-image-availability-exporter

[![Docker images](https://img.shields.io/docker/automated/flant/k8s-image-availability-exporter)](https://hub.docker.com/r/flant/k8s-image-availability-exporter)
[![Latest Docker image](https://img.shields.io/docker/v/flant/k8s-image-availability-exporter?sort=semver)](https://hub.docker.com/r/flant/k8s-image-availability-exporter)

## Deploying

After cloning this repo:

`kubectl apply -f deploy/`

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

* `--bind-address` — IP address and port to bind to.
  * Default: `:8080`
* `--check-interval` — interval for checking absent images. In Go `time` format.
  * Default: `5m`
* `--ignored-images` — comma-separated list of images to ignore while checking absent images.
* `--skip-registry-cert-verification` — whether to skip registries' certificate verification.

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

`k8s-image-availability-exporter` is compatible with Kubernetes 1.15+ and Docker Registry V2 compliant container registries.

Since the exporter operates as a Deployment, it *does not* support container registries that should be accessed via authorization on a node.
