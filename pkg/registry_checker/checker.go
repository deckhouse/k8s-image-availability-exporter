package registry_checker

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"regexp"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sirupsen/logrus"

	appsv1informers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/informers/batch/v1beta1"
	corev1informers "k8s.io/client-go/informers/core/v1"

	"k8s.io/client-go/informers"

	"k8s.io/client-go/kubernetes"

	"github.com/flant/k8s-image-availability-exporter/pkg/store"
)

const (
	resyncPeriod         = time.Hour
	failedCheckBatchSize = 20
	checkBatchSize       = 50
)

type registryCheckerConfig struct {
	defaultRegistry string
}

type RegistryChecker struct {
	imageStore *store.ImageStore

	deploymentsInformer  appsv1informers.DeploymentInformer
	statefulSetsInformer appsv1informers.StatefulSetInformer
	daemonSetsInformer   appsv1informers.DaemonSetInformer
	cronJobsInformer     v1beta1.CronJobInformer
	secretsInformer      corev1informers.SecretInformer

	controllerIndexers ControllerIndexers

	ignoredImagesRegex []regexp.Regexp

	registryTransport *http.Transport

	kubeClient *kubernetes.Clientset

	config registryCheckerConfig
}

func NewRegistryChecker(
	stopCh <-chan struct{},
	kubeClient *kubernetes.Clientset,
	skipVerify bool,
	ignoredImages []regexp.Regexp,
	specificNamespace string,
	defaultRegistry string,
) *RegistryChecker {

	informerFactory := informers.NewSharedInformerFactory(kubeClient, time.Hour)
	if specificNamespace != "" {
		informerFactory = informers.NewSharedInformerFactoryWithOptions(kubeClient, time.Hour, informers.WithNamespace(specificNamespace))
	}

	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	if skipVerify {
		customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	rc := &RegistryChecker{
		deploymentsInformer:  informerFactory.Apps().V1().Deployments(),
		statefulSetsInformer: informerFactory.Apps().V1().StatefulSets(),
		daemonSetsInformer:   informerFactory.Apps().V1().DaemonSets(),
		cronJobsInformer:     informerFactory.Batch().V1beta1().CronJobs(),
		secretsInformer:      informerFactory.Core().V1().Secrets(),

		ignoredImagesRegex: ignoredImages,

		registryTransport: customTransport,

		kubeClient: kubeClient,

		config: registryCheckerConfig{
			defaultRegistry: defaultRegistry,
		},
	}

	rc.imageStore = store.NewImageStore(rc.Check, checkBatchSize, failedCheckBatchSize)

	rc.deploymentsInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			rc.reconcile(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			rc.reconcileUpdate(oldObj, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			rc.reconcile(obj)
		},
	}, resyncPeriod)
	err := rc.deploymentsInformer.Informer().AddIndexers(deploymentIndexers)
	if err != nil {
		panic(err)
	}
	rc.controllerIndexers.deploymentIndexer = rc.deploymentsInformer.Informer().GetIndexer()
	go rc.deploymentsInformer.Informer().Run(stopCh)

	rc.statefulSetsInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			rc.reconcile(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			rc.reconcileUpdate(oldObj, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			rc.reconcile(obj)
		},
	}, resyncPeriod)
	err = rc.statefulSetsInformer.Informer().AddIndexers(statefulSetIndexers)
	if err != nil {
		panic(err)
	}
	rc.controllerIndexers.statefulSetIndexer = rc.statefulSetsInformer.Informer().GetIndexer()
	go rc.statefulSetsInformer.Informer().Run(stopCh)

	rc.daemonSetsInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			rc.reconcile(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			rc.reconcileUpdate(oldObj, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			rc.reconcile(obj)
		},
	}, resyncPeriod)
	err = rc.daemonSetsInformer.Informer().AddIndexers(daemonSetIndexers)
	if err != nil {
		panic(err)
	}
	rc.controllerIndexers.daemonSetIndexer = rc.daemonSetsInformer.Informer().GetIndexer()
	go rc.daemonSetsInformer.Informer().Run(stopCh)

	rc.cronJobsInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			rc.reconcile(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			rc.reconcileUpdate(oldObj, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			rc.reconcile(obj)
		},
	}, resyncPeriod)
	err = rc.cronJobsInformer.Informer().AddIndexers(cronJobIndexers)
	if err != nil {
		panic(err)
	}
	rc.controllerIndexers.cronJobIndexer = rc.cronJobsInformer.Informer().GetIndexer()
	go rc.cronJobsInformer.Informer().Run(stopCh)

	rc.controllerIndexers.secretIndexer = rc.secretsInformer.Informer().GetIndexer()
	go rc.secretsInformer.Informer().Run(stopCh)

	logrus.Info("Waiting for cache sync")
	cache.WaitForCacheSync(stopCh, rc.deploymentsInformer.Informer().HasSynced, rc.statefulSetsInformer.Informer().HasSynced,
		rc.daemonSetsInformer.Informer().HasSynced, rc.cronJobsInformer.Informer().HasSynced, rc.secretsInformer.Informer().HasSynced)
	logrus.Info("Caches populated successfully")

	rc.imageStore.RunGC(rc.controllerIndexers.GetContainerInfosForImage)

	return rc
}

// Collect implements prometheus.Collector.
func (rc *RegistryChecker) Collect(ch chan<- prometheus.Metric) {
	metrics := rc.imageStore.ExtractMetrics()

	for _, m := range metrics {
		ch <- m
	}
}

// Describe implements prometheus.Collector.
func (rc *RegistryChecker) Describe(_ chan<- *prometheus.Desc) {}

func (rc RegistryChecker) Tick() {
	rc.imageStore.Check()
}

func (rc *RegistryChecker) reconcile(obj interface{}) {
	images := ExtractImages(obj)

imagesLoop:
	for _, image := range images {
		for _, ignoredImageRegex := range rc.ignoredImagesRegex {
			if ignoredImageRegex.MatchString(image) {
				continue imagesLoop
			}
		}

		var skipObject bool

		switch typedObj := obj.(type) {
		case *appsv1.Deployment:
			if typedObj.Status.Replicas == 0 {
				skipObject = true
			}
		case *appsv1.StatefulSet:
			if typedObj.Status.Replicas == 0 {
				skipObject = true
			}
		case *appsv1.DaemonSet:
			if typedObj.Status.CurrentNumberScheduled == 0 {
				skipObject = true
			}
		}

		if skipObject {
			rc.imageStore.ReconcileImage(image, []store.ContainerInfo{})
			continue
		}

		containerInfos := rc.controllerIndexers.GetContainerInfosForImage(image)
		rc.imageStore.ReconcileImage(image, containerInfos)
	}
}

func (rc *RegistryChecker) reconcileUpdate(a, b interface{}) {
	if EqualObjects(a, b) {
		return
	}

	rc.reconcile(b)
}

func (rc *RegistryChecker) Check(imageName string) store.AvailabilityMode {
	keyChain := rc.controllerIndexers.GetKeychainForImage(rc.kubeClient, imageName)

	log := logrus.WithField("image_name", imageName)
	return rc.checkImageAvailability(log, imageName, keyChain)
}

func (rc *RegistryChecker) checkImageAvailability(log *logrus.Entry, imageName string, kc *keychain) (availMode store.AvailabilityMode) {
	ref, err := parseImageName(imageName, rc.config.defaultRegistry)
	if err != nil {
		return checkImageNameParseErr(log, err)
	}

	imgErr := wait.ExponentialBackoff(wait.Backoff{
		Duration: time.Second,
		Factor:   2,
		Steps:    2,
	}, func() (bool, error) {
		var err error
		availMode, err = check(ref, kc, rc.registryTransport)

		return availMode == store.Available, err
	})

	if availMode != store.Available {
		log.WithField("availability_mode", availMode.String()).Error(imgErr)
	}

	return
}

func checkImageNameParseErr(log *logrus.Entry, err error) store.AvailabilityMode {
	var parseErr *name.ErrBadName
	if errors.As(err, &parseErr) {
		log.WithField("availability_mode", store.BadImageName.String()).Error(err)
		return store.BadImageName
	}

	log.WithField("availability_mode", store.UnknownError.String()).Error(err)
	return store.UnknownError
}

func parseImageName(image string, defaultRegistry string) (name.Reference, error) {
	var (
		ref name.Reference
		err error
	)

	if len(defaultRegistry) == 0 {
		ref, err = name.ParseReference(image)
	} else {
		ref, err = name.ParseReference(image, name.WithDefaultRegistry(defaultRegistry))
	}
	if err != nil {
		return nil, err
	}

	return ref, nil
}

func check(ref name.Reference, kc *keychain, registryTransport *http.Transport) (store.AvailabilityMode, error) {
	var imgErr error

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if kc != nil {
		for i := 0; i < kc.size; i++ {
			kc.index = i

			_, imgErr = remote.Head(ref, remote.WithAuthFromKeychain(kc), remote.WithTransport(registryTransport),
				remote.WithContext(ctx))

			if imgErr == nil || (!IsAuthnFail(imgErr) && !IsAuthzFail(imgErr)) {
				break
			}
		}

		kc.index = 0
	} else {
		_, imgErr = remote.Head(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithTransport(registryTransport), remote.WithContext(ctx))
	}

	var availMode store.AvailabilityMode
	if IsAbsent(imgErr) {
		availMode = store.Absent
	} else if IsAuthnFail(imgErr) {
		availMode = store.AuthnFailure
	} else if IsAuthzFail(imgErr) {
		availMode = store.AuthzFailure
	} else if IsOldRegistry(imgErr) {
		availMode = store.Available
	} else if imgErr != nil {
		availMode = store.UnknownError
	}

	return availMode, imgErr
}
