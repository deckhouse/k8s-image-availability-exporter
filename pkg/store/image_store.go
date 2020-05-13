package store

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/emirpasic/gods/trees/binaryheap"
)

type AvailabilityMode int

const (
	Available AvailabilityMode = iota
	Absent
	BadImageName
	RegistryUnavailable
	AuthnFailure
	AuthzFailure
	V1Schema
	UnknownError
)

func AvailabilityModeDescMap() map[AvailabilityMode]string {
	return map[AvailabilityMode]string{
		Available:           "available",
		Absent:              "absent",
		BadImageName:        "bad_image_format",
		RegistryUnavailable: "registry_unavailable",
		AuthnFailure:        "authentication_failure",
		AuthzFailure:        "authorization_failure",
		V1Schema:            "registry_v1_api_not_supported",
		UnknownError:        "unknown_error",
	}
}

func (a AvailabilityMode) String() string {
	return AvailabilityModeDescMap()[a]
}

type ContainerInfo struct {
	Namespace      string
	ControllerKind string
	ControllerName string
	Container      string
}

type ImageInfo struct {
	Info      map[ContainerInfo]struct{}
	LastCheck time.Time
	AvailMode AvailabilityMode
}

type imageLastCheckPair struct {
	image     string
	lastCheck time.Time
}

type ImageStore struct {
	lock sync.RWMutex

	imageSet map[string]ImageInfo
	pq       *binaryheap.Heap
}

func NewImageStore() *ImageStore {
	return &ImageStore{
		imageSet: map[string]ImageInfo{},
		pq:       binaryheap.NewWith(compareTimeInPair),
	}
}

func (imgStore *ImageStore) UpdateContainerAssociations(image string, containerInfos []ContainerInfo) {
	var containerInfoMap = map[ContainerInfo]struct{}{}

	for _, info := range containerInfos {
		containerInfoMap[info] = struct{}{}
	}

	imgStore.lock.Lock()
	imageInfo := imgStore.imageSet[image]
	imageInfo.Info = containerInfoMap
	imgStore.imageSet[image] = imageInfo
	imgStore.lock.Unlock()
}

func (imgStore *ImageStore) RemoveContainerAssociation(image, namespace, kind, name, container string) {
	imgStore.lock.Lock()
	defer imgStore.lock.Unlock()

	delete(imgStore.imageSet[image].Info, ContainerInfo{
		Namespace:      namespace,
		ControllerKind: kind,
		ControllerName: name,
		Container:      container,
	})
}

func (imgStore *ImageStore) ExtractMetrics() (ret []prometheus.Metric) {
	imgStore.lock.RLock()
	defer imgStore.lock.RUnlock()

	for imageName, image := range imgStore.imageSet {
		for k := range image.Info {
			if image.LastCheck.IsZero() {
				continue
			}
			ret = append(ret, newNamedConstMetrics(k.ControllerKind, k.ControllerName, k.Namespace, k.Container, imageName, image.AvailMode)...)
		}
	}

	return
}

func (imgStore *ImageStore) AddOrUpdateImage(imageName string, lastCheck time.Time, availMode ...AvailabilityMode) {
	imgStore.lock.Lock()
	defer imgStore.lock.Unlock()

	var mode AvailabilityMode
	if len(availMode) != 0 {
		mode = availMode[0]
	}

	if _, ok := imgStore.imageSet[imageName]; ok && lastCheck.IsZero() {
		return
	} else if !ok {
		imgStore.imageSet[imageName] = ImageInfo{
			Info:      map[ContainerInfo]struct{}{},
			LastCheck: lastCheck,
			AvailMode: mode,
		}
		imgStore.pq.Push(imageLastCheckPair{
			image:     imageName,
			lastCheck: lastCheck,
		})
	} else {
		imageInfo := imgStore.imageSet[imageName]
		imageInfo.LastCheck = lastCheck
		imageInfo.AvailMode = mode
		imgStore.imageSet[imageName] = imageInfo

		imgStore.pq.Push(imageLastCheckPair{
			image:     imageName,
			lastCheck: lastCheck,
		})
	}
}

func (imgStore *ImageStore) RemoveImage(imageName string) {
	imgStore.lock.Lock()
	defer imgStore.lock.Unlock()

	delete(imgStore.imageSet, imageName)
}

func (imgStore *ImageStore) Length() int {
	return len(imgStore.imageSet)
}

func (imgStore *ImageStore) PopOldestImages(count int) (ret []string) {
	imgStore.lock.Lock()
	imgStore.lock.Unlock()

	if count == 0 {
		count = 1
	}

	for i := 0; i < count; i++ {
		value, exists := imgStore.pq.Pop()
		if !exists {
			break
		}

		pair := value.(imageLastCheckPair)
		// lazily remove pair from priority queue if an image doesn't exist in the imageSet
		if _, ok := imgStore.imageSet[pair.image]; !ok {
			continue
		}

		ret = append(ret, pair.image)
	}

	return
}

func compareTimeInPair(a, b interface{}) int {
	first := a.(imageLastCheckPair)
	second := b.(imageLastCheckPair)

	switch {
	case first.lastCheck.Before(second.lastCheck):
		return -1
	case first.lastCheck.After(second.lastCheck):
		return 1
	default:
		return 0
	}
}

func newNamedConstMetrics(ownerKind, ownerName, namespace, container, image string, avalMode AvailabilityMode) (ret []prometheus.Metric) {
	labels := map[string]string{
		"namespace": namespace,
		"container": container,
		"image":     image,
	}

	switch ownerKind {
	case "Deployment":
		labels["deployment"] = ownerName
		return getMetricByControllerKind(ownerKind, labels, avalMode)
	case "StatefulSet":
		labels["statefulset"] = ownerName
		return getMetricByControllerKind(ownerKind, labels, avalMode)
	case "DaemonSet":
		labels["daemonset"] = ownerName
		return getMetricByControllerKind(ownerKind, labels, avalMode)
	case "CronJob":
		labels["cronjob"] = ownerName
		return getMetricByControllerKind(ownerKind, labels, avalMode)
	default:
		panic(fmt.Sprintf("received unknown metric name: %s", ownerKind))
	}
}

func getMetricByControllerKind(controllerKind string, labels map[string]string, mode AvailabilityMode) (ret []prometheus.Metric) {
	for availMode, desc := range AvailabilityModeDescMap() {
		var value float64
		if availMode == mode {
			value = 1
		}

		ret = append(ret, prometheus.MustNewConstMetric(
			prometheus.NewDesc("k8s_image_existence_exporter_"+strings.ToLower(controllerKind)+"_"+desc, "", nil, labels),
			prometheus.GaugeValue,
			value,
		))
	}

	return
}
