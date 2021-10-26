package store

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gammazero/deque"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/util/wait"
)

type AvailabilityMode int

const (
	Available AvailabilityMode = iota
	Absent
	BadImageName
	RegistryUnavailable
	AuthnFailure
	AuthzFailure
	UnknownError
)

var AvailabilityModeDescMap = map[AvailabilityMode]string{
	Available:           "available",
	Absent:              "absent",
	BadImageName:        "bad_image_format",
	RegistryUnavailable: "registry_unavailable",
	AuthnFailure:        "authentication_failure",
	AuthzFailure:        "authorization_failure",
	UnknownError:        "unknown_error",
}

func (a AvailabilityMode) String() string {
	return AvailabilityModeDescMap[a]
}

type ContainerInfo struct {
	Namespace      string
	ControllerKind string
	ControllerName string
	Container      string
}

type ImageInfo struct {
	ContainerInfo map[ContainerInfo]struct{}
	AvailMode     AvailabilityMode
}

type ImageStore struct {
	lock sync.RWMutex

	imageSet map[string]ImageInfo
	queue    *deque.Deque
	errQueue *deque.Deque

	check checkFunc

	concurrentNormalChecks int
	concurrentErrorChecks  int
}

type checkFunc func(imageName string) AvailabilityMode
type gcFunc func(image string) []ContainerInfo

func NewImageStore(check checkFunc, concurrentNormalChecks, concurrentErrorChecks int) *ImageStore {
	return &ImageStore{
		imageSet: make(map[string]ImageInfo),
		queue:    deque.New(2048, 2048),
		errQueue: deque.New(512, 512),

		check: check,

		concurrentNormalChecks: concurrentNormalChecks,
		concurrentErrorChecks:  concurrentErrorChecks,
	}
}

func (s *ImageStore) RunGC(gc gcFunc) {
	go wait.Forever(func() {
		s.lock.Lock()
		defer s.lock.Unlock()

		for image, imgInfo := range s.imageSet {
			ci := gc(image)

			if len(ci) == 0 {
				delete(s.imageSet, image)

				continue
			}

			imgInfo.ContainerInfo = containerInfoSliceToSet(ci)
			s.imageSet[image] = imgInfo
		}

	}, 15*time.Minute)
}

func (s *ImageStore) ExtractMetrics() (ret []prometheus.Metric) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	for imageName, info := range s.imageSet {
		for containerInfo := range info.ContainerInfo {
			ret = append(ret, newNamedConstMetrics(containerInfo.ControllerKind, containerInfo.ControllerName,
				containerInfo.Namespace, containerInfo.Container, imageName, info.AvailMode)...)
		}
	}

	return
}

func (s *ImageStore) ReconcileImage(imageName string, containerInfos []ContainerInfo) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if len(containerInfos) == 0 {
		delete(s.imageSet, imageName)

		return
	}

	imageInfo, ok := s.imageSet[imageName]
	if !ok {
		containerInfoMap := containerInfoSliceToSet(containerInfos)

		s.imageSet[imageName] = ImageInfo{ContainerInfo: containerInfoMap}
		s.queue.PushBack(imageName)

		return
	}

	for _, ci := range containerInfos {
		imageInfo.ContainerInfo[ci] = struct{}{}
	}

	s.imageSet[imageName] = imageInfo
}

func (s *ImageStore) Check() {
	s.lock.Lock()
	defer s.lock.Unlock()

	var (
		normalChecks = s.concurrentNormalChecks
		errChecks    = s.concurrentErrorChecks
	)

	if qLen := s.queue.Len(); qLen < s.concurrentNormalChecks {
		normalChecks = qLen
	}
	if qLen := s.errQueue.Len(); qLen < s.concurrentErrorChecks {
		errChecks = qLen
	}

	errPops := s.popCheckPush(true, errChecks)

	if errPops < errChecks {
		normalChecks += errChecks - errPops
	}

	_ = s.popCheckPush(false, normalChecks)
}

func (s *ImageStore) popCheckPush(errQ bool, count int) (pops int) {
	for pops < count {
		var imageRaw interface{}
		if errQ {
			imageRaw = s.errQueue.PopFront()
		} else {
			imageRaw = s.queue.PopFront()
		}

		image := imageRaw.(string)

		availMode := s.check(image)

		imageInfo, ok := s.imageSet[image]
		if !ok {
			continue
		}

		imageInfo.AvailMode = availMode

		s.imageSet[image] = imageInfo

		if availMode == Available {
			s.queue.PushBack(image)
		} else {
			s.errQueue.PushBack(image)
		}

		pops++
	}

	return
}

func containerInfoSliceToSet(containerInfos []ContainerInfo) map[ContainerInfo]struct{} {
	var containerInfoMap = make(map[ContainerInfo]struct{})
	for _, ci := range containerInfos {
		containerInfoMap[ci] = struct{}{}
	}

	return containerInfoMap
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
	for availMode, desc := range AvailabilityModeDescMap {
		var value float64
		if availMode == mode {
			value = 1
		}

		ret = append(ret, prometheus.MustNewConstMetric(
			prometheus.NewDesc("k8s_image_availability_exporter_"+strings.ToLower(controllerKind)+"_"+desc, "", nil, labels),
			prometheus.GaugeValue,
			value,
		))
	}

	return
}
