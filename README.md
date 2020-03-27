# k8s-image-existence-exporter

[![Docker images](https://img.shields.io/docker/automated/flant/k8s-image-existence-exporter)](https://hub.docker.com/r/flant/k8s-image-existence-exporter)
[![Latest Docker image](https://img.shields.io/docker/v/flant/k8s-image-existence-exporter?sort=semver)](https://hub.docker.com/r/flant/k8s-image-existence-exporter)

## Deploying

After cloning this repo:

`kubectl apply -f deploy/`

### Prometheus integration
 
Here's how you can configure Prometheus or prometheus-operator to scrape metrics from `k8s-image-existence-operator`.
 
#### Prometheus

```yaml
- job_name: image-existence-exporter
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
    regex: image-existence-exporter
    action: keep
```

#### prometheus-operator

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: image-existence-exporter
  namespace: kube-system
spec:
  podMetricsEndpoints:
  - port: http-metrics
    scheme: http
    honorLabels: true
    scrapeTimeout: 10s
  selector:
    matchLabels:
      app: image-existence-exporter
  namespaceSelector:
    matchNames:
    - kube-system
```

### Alerting

And alert on them.

#### Prometheus

```yaml
groups:
- name: image-existence-exporter.rules
  rules:
  - alert: DeploymentImageAbsent
    expr: |
      max by (namespace, owner_name, container, image) (
        k8s_image_existence_exporter_deployment_image_exists == 0
      )
    annotations:
      description: >
        Check image's `{{ $labels.image }}` existence in container registry
        in Namespace `{{ $labels.namespace }}`
        in Deployment `{{ $labels.owner_name }}`
        in container `{{ $labels.container }}` в registry.
      summary: Image `{{ $labels.image }}` is absent from container registry.

  - alert: StatefulSetImageAbsent
    expr: |
      max by (namespace, owner_name, container, image) (
        k8s_image_existence_exporter_statefulset_image_exists == 0
      )
    annotations:
      description: >
        Check image's `{{ $labels.image }}` existence in container registry
        in Namespace `{{ $labels.namespace }}`
        in Deployment `{{ $labels.owner_name }}`
        in container `{{ $labels.container }}` в registry.
      summary: Image `{{ $labels.image }}` is absent from container registry.

  - alert: DaemonSetImageAbsent
    expr: |
      max by (namespace, owner_name, container, image) (
        k8s_image_existence_exporter_daemonset_image_exists == 0
      )
    annotations:
      description: >
        Check image's `{{ $labels.image }}` existence in container registry
        in Namespace `{{ $labels.namespace }}`
        in Deployment `{{ $labels.owner_name }}`
        in container `{{ $labels.container }}` в registry.
      summary: Image `{{ $labels.image }}` is absent from container registry.

  - alert: CronJobImageAbsent
    expr: |
      max by (namespace, owner_name, container, image) (
        k8s_image_existence_exporter_cronjob_image_exists == 0
      )
    annotations:
      description: >
        Check image's `{{ $labels.image }}` existence in container registry
        in Namespace `{{ $labels.namespace }}`
        in Deployment `{{ $labels.owner_name }}`
        in container `{{ $labels.container }}` в registry.
      summary: Image `{{ $labels.image }}` is absent from container registry.
```

#### prometheus-operator

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: image-existence-exporter-alerts
  namespace: kube-system
spec:
  groups:
  - name: image-existence-exporter.rules
    rules:

    - alert: DeploymentImageAbsent
      expr: |
        max by (namespace, owner_name, container, image) (
          k8s_image_existence_exporter_deployment_image_exists == 0
        )
      annotations:
        description: >
          Check image's `{{ $labels.image }}` existence in container registry
          in Namespace `{{ $labels.namespace }}`
          in Deployment `{{ $labels.owner_name }}`
          in container `{{ $labels.container }}` в registry.
        summary: Image `{{ $labels.image }}` is absent from container registry.
    
    - alert: StatefulSetImageAbsent
      expr: |
        max by (namespace, owner_name, container, image) (
          k8s_image_existence_exporter_statefulset_image_exists == 0
        )
      annotations:
        description: >
          Check image's `{{ $labels.image }}` existence in container registry
          in Namespace `{{ $labels.namespace }}`
          in Deployment `{{ $labels.owner_name }}`
          in container `{{ $labels.container }}` в registry.
        summary: Image `{{ $labels.image }}` is absent from container registry.
    
    - alert: DaemonSetImageAbsent
      expr: |
        max by (namespace, owner_name, container, image) (
          k8s_image_existence_exporter_daemonset_image_exists == 0
        )
      annotations:
        description: >
          Check image's `{{ $labels.image }}` existence in container registry
          in Namespace `{{ $labels.namespace }}`
          in Deployment `{{ $labels.owner_name }}`
          in container `{{ $labels.container }}` в registry.
        summary: Image `{{ $labels.image }}` is absent from container registry.
    
    - alert: CronJobImageAbsent
      expr: |
        max by (namespace, owner_name, container, image) (
          k8s_image_existence_exporter_cronjob_image_exists == 0
        )
      annotations:
        description: >
          Check image's `{{ $labels.image }}` existence in container registry
          in Namespace `{{ $labels.namespace }}`
          in Deployment `{{ $labels.owner_name }}`
          in container `{{ $labels.container }}` в registry.
        summary: Image `{{ $labels.image }}` is absent from container registry.
```

## Configuration

### Command-line options

* `--bind-address` — IP address and port to bind to.
  * Default: `:8080`
* `--check-interval` — interval for checking absent images. In Go `time` format.
  * Default: `5m`
* `--ignored-images` — comma-separated list of images to ignore while checking absent images.

## Metrics

* `k8s_image_existence_exporter_deployment_image_exists` — contains **0** for absent, **1** for present image in container registry along with labels that identify the Deployment whose `image` was checked.
* `k8s_image_existence_exporter_statefulset_image_exists` — contains **0** for absent, **1** for present image in container registry along with labels that identify the StatefulSet whose `image` was checked.
* `k8s_image_existence_exporter_daemonset_image_exists` — contains **0** for absent, **1** for present image in container registry along with labels that identify the DaemonSet whose `image` was checked.
* `k8s_image_existence_exporter_cronjob_image_exists` — contains **0** for absent, **1** for present image in container registry along with labels that identify the CronJob whose `image` was checked.
* `k8s_image_existence_exporter_completed_rechecks_total` — increments every `--check-interval`.
* `k8s_image_existence_exporter_generic_errors_total` — increments every time a single image check returns an error.