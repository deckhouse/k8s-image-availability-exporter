package store

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func insertImagesIntoStore(t *testing.T, store *ImageStore, successfulChecks, failedChecks int) {
	t.Helper()

	dummyContainerInfos := []ContainerInfo{
		{
			Namespace:      "test",
			ControllerKind: "Deployment",
			ControllerName: "test",
			Container:      "test",
		},
	}

	for i := 0; i < successfulChecks; i++ {
		store.ReconcileImage(fmt.Sprintf("test_%d", i), dummyContainerInfos)
	}
	for i := 0; i < failedChecks; i++ {
		store.ReconcileImage(fmt.Sprintf("fail_%d", i), dummyContainerInfos)
	}
}

func TestImageStore_AddOrUpdateImage(t *testing.T) {
	store := NewImageStore(reconcile(t), 2, 3)

	insertImagesIntoStore(t, store, 3, 2)

	store.Check()

	metrics := store.ExtractMetrics()
	require.Len(t, metrics, 35)
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
