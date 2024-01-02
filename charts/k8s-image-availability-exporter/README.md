# k8s-image-availability-exporter

![Version: 0.8.0](https://img.shields.io/badge/Version-0.8.0-informational?style=flat-square) ![AppVersion: v0.5.1](https://img.shields.io/badge/AppVersion-v0.5.1-informational?style=flat-square)

Application for monitoring the cluster workloads image presence in a container registry.

## Requirements

Kubernetes: `>=1.14.0-0`

## Introduction

This chart bootstraps a [k8s-image-availability-exporter](https://github.com/flant/k8s-image-availability-exporter) deployment on a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| k8sImageAvailabilityExporter.image.repository | string | `"registry.deckhouse.io/k8s-image-availability-exporter/k8s-image-availability-exporter"` | Repository to use for the k8s-image-availability-exporter deployment |
| k8sImageAvailabilityExporter.image.tag | string | `""` | Image tag override for the default value (chart appVersion) |
| k8sImageAvailabilityExporter.image.pullPolicy | string | `"IfNotPresent"` | Image pull policy to use for the k8s-image-availability-exporter deployment |
| k8sImageAvailabilityExporter.args | list | `["--bind-address=:8080"]` | Command line arguments for the exporter |
| replicaCount | int | `1` | Number of replicas (pods) to launch. |
| podSecurityContext | object | `{}` | Pod [security context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod). See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#security-context) for details. |
| securityContext | object | `{}` | Container [security context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-container). See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#security-context-1) for details. |
| nodeSelector | object | `{}` | [Node selector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector) configuration. |
| tolerations | list | `[]` | [Tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) for node taints. See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#scheduling) for details. |
| affinity | object | `{}` | [Affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity) configuration. See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#scheduling) for details. |
| topologySpreadConstraints | list | `[]` | [TopologySpreadConstraints](https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/) configuration. See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#scheduling) for details. |
| strategy | object | `{}` | Deployment [strategy](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#strategy) configuration. |
| revisionHistoryLimit | int | `10` | Define the [count of deployment revisions](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#clean-up-policy) to be kept. May be set to 0 in case of GitOps deployment approach. |
| podDisruptionBudget.enabled | bool | `false` | Enable a [pod distruption budget](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) to help dealing with [disruptions](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/). It is **highly recommended** for webhooks as disruptions can prevent launching new pods. |
| podDisruptionBudget.minAvailable | int/percentage | `nil` | Number or percentage of pods that must remain available. |
| podDisruptionBudget.maxUnavailable | int/percentage | `nil` | Number or percentage of pods that can be unavailable. |
| priorityClassName | string | `""` | Specify a priority class name to set [pod priority](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#pod-priority). |
| resources | object | No requests or limits. | Container resource [requests and limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/). See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#resources) for details. |
| serviceMonitor.enabled | bool | `false` | Create [Prometheus Operator](https://github.com/coreos/prometheus-operator) serviceMonitor resource |
| serviceMonitor.interval | string | `"15s"` | Scrape interval for serviceMonitor |
| prometheusRule.enabled | bool | `false` | Create [Prometheus Operator](https://github.com/coreos/prometheus-operator) prometheusRule resource |
| prometheusRule.defaultGroupsEnabled | bool | `true` | Setup default alerts (works only if prometheusRule.enabled is set to true) |
| prometheusRule.additionalGroups | list | `[]` | Additional PrometheusRule groups |

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example,

```bash
helm install my-release k8s-image-availability-exporter --set k8sImageAvailabilityExporter.replicas=2
```

Alternatively, one or more YAML files that specify the values for the above parameters can be provided while installing the chart. For example,

```bash
helm install my-release k8s-image-availability-exporter -f values1.yaml,values2.yaml
```
