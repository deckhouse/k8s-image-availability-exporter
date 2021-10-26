package store

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var dummyContainerInfos = []ContainerInfo{
	{
		Namespace:      "test",
		ControllerKind: "Deployment",
		ControllerName: "test",
		Container:      "test",
	},
}

func TestImageStore_AddOrUpdateImage(t *testing.T) {
	store := NewImageStore(reconcile(t), 2, 3)

	store.ReconcileImage("fail_1", dummyContainerInfos)
	store.ReconcileImage("fail_2", dummyContainerInfos)
	store.ReconcileImage("test_1", dummyContainerInfos)
	store.ReconcileImage("test_2", dummyContainerInfos)
	store.ReconcileImage("test_3", dummyContainerInfos)

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
