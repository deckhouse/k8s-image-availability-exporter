package store

import (
	"fmt"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertImagesIntoStore(t *testing.T, store *ImageStore, successfulChecks, failedChecks int, info []ContainerInfo) {
	t.Helper()

	for i := 0; i < successfulChecks; i++ {
		store.ReconcileImage(fmt.Sprintf("test_%d", i), info)
	}
	for i := 0; i < failedChecks; i++ {
		store.ReconcileImage(fmt.Sprintf("fail_%d", i), info)
	}
}

func TestImageStore_AddOrUpdateImage(t *testing.T) {
	store := NewImageStore(reconcile(t), 2, 3)

	info := []ContainerInfo{
		{
			Namespace:      "test",
			ControllerKind: "Deployment",
			ControllerName: "test",
			Container:      "test",
		},
		{
			Namespace:      "test",
			ControllerKind: "StatefulSet",
			ControllerName: "test",
			Container:      "test",
		},
	}

	insertImagesIntoStore(t, store, 3, 2, info)

	store.Check()

	metrics := store.ExtractMetrics()
	require.Len(t, metrics, 70)
}

func reconcile(t *testing.T) func(imageName string) AvailabilityMode {
	t.Helper()

	return func(imageName string) AvailabilityMode {
		if strings.HasPrefix(imageName, "fail_") {
			return UnknownError
		}

		return Available
	}
}

func TestImageStore_ExtractMetrics(t *testing.T) {
	t.Parallel()

	t.Run("no images", func(t *testing.T) {
		t.Parallel()

		store := NewImageStore(reconcile(t), 2, 3)
		insertImagesIntoStore(t, store, 0, 0, nil)
		store.Check()

		metrics := store.ExtractMetrics()
		assert.Empty(t, metrics)
	})

	t.Run("one container", func(t *testing.T) {
		t.Parallel()

		store := NewImageStore(reconcile(t), 2, 3)

		info := []ContainerInfo{
			{
				Namespace:      "test_ns",
				ControllerKind: "Deployment",
				ControllerName: "test_name",
				Container:      "test_container",
			},
		}

		expectedMetrics := []*prometheus.Desc{
			prometheus.NewDesc(
				"k8s_image_availability_exporter_registry_unavailable",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container",
					"image":     "test_0",
					"kind":      "deployment",
					"name":      "test_name",
					"namespace": "test_ns",
				},
			),
			prometheus.NewDesc(
				"k8s_image_availability_exporter_authentication_failure",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container",
					"image":     "test_0",
					"kind":      "deployment",
					"name":      "test_name",
					"namespace": "test_ns",
				},
			),
			prometheus.NewDesc(
				"k8s_image_availability_exporter_authorization_failure",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container",
					"image":     "test_0",
					"kind":      "deployment",
					"name":      "test_name",
					"namespace": "test_ns",
				},
			),
			prometheus.NewDesc(
				"k8s_image_availability_exporter_unknown_error",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container",
					"image":     "test_0",
					"kind":      "deployment",
					"name":      "test_name",
					"namespace": "test_ns",
				},
			),
			prometheus.NewDesc(
				"k8s_image_availability_exporter_available",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container",
					"image":     "test_0",
					"kind":      "deployment",
					"name":      "test_name",
					"namespace": "test_ns",
				},
			),
			prometheus.NewDesc(
				"k8s_image_availability_exporter_absent",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container",
					"image":     "test_0",
					"kind":      "deployment",
					"name":      "test_name",
					"namespace": "test_ns",
				},
			),
			prometheus.NewDesc(
				"k8s_image_availability_exporter_bad_image_format",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container",
					"image":     "test_0",
					"kind":      "deployment",
					"name":      "test_name",
					"namespace": "test_ns",
				},
			),
		}

		insertImagesIntoStore(t, store, 1, 0, info)
		store.Check()

		metrics := store.ExtractMetrics()
		require.Len(t, metrics, len(expectedMetrics))

		expectedMetricsStr := make([]string, 0, len(expectedMetrics))
		for _, em := range expectedMetrics {
			expectedMetricsStr = append(expectedMetricsStr, em.String())
		}

		returnedMetricsStr := make([]string, 0, len(metrics))
		for _, m := range metrics {
			returnedMetricsStr = append(returnedMetricsStr, m.Desc().String())
		}

		assert.ElementsMatch(t, expectedMetricsStr, returnedMetricsStr)
	})

	t.Run("two containers, different kind", func(t *testing.T) {
		t.Parallel()

		store := NewImageStore(reconcile(t), 2, 3)

		info := []ContainerInfo{
			{
				Namespace:      "test_ns",
				ControllerKind: "Deployment",
				ControllerName: "test_name",
				Container:      "test_container",
			},
			{
				Namespace:      "test_ns2",
				ControllerKind: "StatefulSet",
				ControllerName: "test_name2",
				Container:      "test_container2",
			},
		}

		expectedMetrics := []*prometheus.Desc{
			prometheus.NewDesc(
				"k8s_image_availability_exporter_registry_unavailable",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container",
					"image":     "test_0",
					"kind":      "deployment",
					"name":      "test_name",
					"namespace": "test_ns",
				},
			),
			prometheus.NewDesc(
				"k8s_image_availability_exporter_authentication_failure",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container",
					"image":     "test_0",
					"kind":      "deployment",
					"name":      "test_name",
					"namespace": "test_ns",
				},
			),
			prometheus.NewDesc(
				"k8s_image_availability_exporter_authorization_failure",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container",
					"image":     "test_0",
					"kind":      "deployment",
					"name":      "test_name",
					"namespace": "test_ns",
				},
			),
			prometheus.NewDesc(
				"k8s_image_availability_exporter_unknown_error",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container",
					"image":     "test_0",
					"kind":      "deployment",
					"name":      "test_name",
					"namespace": "test_ns",
				},
			),
			prometheus.NewDesc(
				"k8s_image_availability_exporter_available",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container",
					"image":     "test_0",
					"kind":      "deployment",
					"name":      "test_name",
					"namespace": "test_ns",
				},
			),
			prometheus.NewDesc(
				"k8s_image_availability_exporter_absent",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container",
					"image":     "test_0",
					"kind":      "deployment",
					"name":      "test_name",
					"namespace": "test_ns",
				},
			),
			prometheus.NewDesc(
				"k8s_image_availability_exporter_bad_image_format",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container",
					"image":     "test_0",
					"kind":      "deployment",
					"name":      "test_name",
					"namespace": "test_ns",
				},
			),
			prometheus.NewDesc(
				"k8s_image_availability_exporter_registry_unavailable",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container2",
					"image":     "test_0",
					"kind":      "statefulset",
					"name":      "test_name2",
					"namespace": "test_ns2",
				},
			),
			prometheus.NewDesc(
				"k8s_image_availability_exporter_authentication_failure",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container2",
					"image":     "test_0",
					"kind":      "statefulset",
					"name":      "test_name2",
					"namespace": "test_ns2",
				},
			),
			prometheus.NewDesc(
				"k8s_image_availability_exporter_authorization_failure",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container2",
					"image":     "test_0",
					"kind":      "statefulset",
					"name":      "test_name2",
					"namespace": "test_ns2",
				},
			),
			prometheus.NewDesc(
				"k8s_image_availability_exporter_unknown_error",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container2",
					"image":     "test_0",
					"kind":      "statefulset",
					"name":      "test_name2",
					"namespace": "test_ns2",
				},
			),
			prometheus.NewDesc(
				"k8s_image_availability_exporter_available",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container2",
					"image":     "test_0",
					"kind":      "statefulset",
					"name":      "test_name2",
					"namespace": "test_ns2",
				},
			),
			prometheus.NewDesc(
				"k8s_image_availability_exporter_absent",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container2",
					"image":     "test_0",
					"kind":      "statefulset",
					"name":      "test_name2",
					"namespace": "test_ns2",
				},
			),
			prometheus.NewDesc(
				"k8s_image_availability_exporter_bad_image_format",
				"",
				nil,
				prometheus.Labels{
					"container": "test_container2",
					"image":     "test_0",
					"kind":      "statefulset",
					"name":      "test_name2",
					"namespace": "test_ns2",
				},
			),
		}

		insertImagesIntoStore(t, store, 1, 0, info)
		store.Check()

		metrics := store.ExtractMetrics()
		require.Len(t, metrics, len(expectedMetrics))

		expectedMetricsStr := make([]string, 0, len(expectedMetrics))
		for _, em := range expectedMetrics {
			expectedMetricsStr = append(expectedMetricsStr, em.String())
		}

		returnedMetricsStr := make([]string, 0, len(metrics))
		for _, m := range metrics {
			returnedMetricsStr = append(returnedMetricsStr, m.Desc().String())
		}

		assert.ElementsMatch(t, expectedMetricsStr, returnedMetricsStr)
	})
}
