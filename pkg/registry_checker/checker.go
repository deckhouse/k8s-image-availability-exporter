package registry_checker

import (
	"errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

	"github.com/sirupsen/logrus"

	appsv1informers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/informers/batch/v1beta1"
	corev1informers "k8s.io/client-go/informers/core/v1"

	"k8s.io/client-go/informers"

	"github.com/flant/k8s-image-availability-exporter/pkg/store"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	resyncPeriod = time.Hour
)

type RegistryChecker struct {
	lock sync.RWMutex

	imageStore *store.ImageStore

	deploymentsInformer   appsv1informers.DeploymentInformer
	statefulSetssInformer appsv1informers.StatefulSetInformer
	daemonSetsInformer    appsv1informers.DaemonSetInformer
	cronJobsInformer      v1beta1.CronJobInformer
	secretsInformer       corev1informers.SecretInformer

	controllerIndexers ControllerIndexers

	ignoredImages map[string]struct{}

	imageExistsVectors []prometheus.Metric
}

func NewRegistryChecker(
	kubeClient *kubernetes.Clientset,
	ignoredImages []string,
) *RegistryChecker {
	informerFactory := informers.NewSharedInformerFactory(kubeClient, time.Hour)

	rc := &RegistryChecker{
		imageStore: store.NewImageStore(),

		deploymentsInformer:   informerFactory.Apps().V1().Deployments(),
		statefulSetssInformer: informerFactory.Apps().V1().StatefulSets(),
		daemonSetsInformer:    informerFactory.Apps().V1().DaemonSets(),
		cronJobsInformer:      informerFactory.Batch().V1beta1().CronJobs(),
		secretsInformer:       informerFactory.Core().V1().Secrets(),

		ignoredImages: map[string]struct{}{},
	}

	for _, image := range ignoredImages {
		rc.ignoredImages[image] = struct{}{}
	}

	return rc
}

func (rc *RegistryChecker) Run(stopCh <-chan struct{}) {
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

	rc.statefulSetssInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
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
	err = rc.statefulSetssInformer.Informer().AddIndexers(statefulSetIndexers)
	if err != nil {
		panic(err)
	}
	rc.controllerIndexers.statefulSetIndexer = rc.statefulSetssInformer.Informer().GetIndexer()
	go rc.statefulSetssInformer.Informer().Run(stopCh)

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
	cache.WaitForCacheSync(stopCh, rc.deploymentsInformer.Informer().HasSynced, rc.statefulSetssInformer.Informer().HasSynced,
		rc.daemonSetsInformer.Informer().HasSynced, rc.cronJobsInformer.Informer().HasSynced, rc.secretsInformer.Informer().HasSynced)
	logrus.Info("Caches populated successfully")
}

func (rc *RegistryChecker) reconcile(obj interface{}) {
	images, err := ExtractImages(obj)
	// TODO: recover?
	if err != nil {
		panic(err)
	}

	for _, image := range images {
		if _, ok := rc.ignoredImages[image]; ok {
			continue
		}

		if !rc.controllerIndexers.CheckImageExistence(image) {
			rc.imageStore.RemoveImage(image)
			continue
		}

		rc.imageStore.AddOrUpdateImage(image, time.Time{})
	}
}

func (rc *RegistryChecker) reconcileUpdate(a, b interface{}) {
	if !EqualObjects(a, b) {
		return
	}

	rc.reconcile(b)
}

func (rc *RegistryChecker) Check() {
	// TODO: tweak const
	oldImages := rc.imageStore.PopOldestImages(rc.imageStore.Length() / 40)

	var processingGroup sync.WaitGroup
	for _, image := range oldImages {
		keyChain := rc.controllerIndexers.GetKeychainForImage(image)

		// TODO: backoff
		processingGroup.Add(1)
		go func(imageName string, kc *keychain) {
			defer processingGroup.Done()

			log := logrus.WithField("image_name", imageName)
			availMode := checkImageAvailability(log, imageName, kc)

			rc.imageStore.AddOrUpdateImage(imageName, time.Now(), availMode)
		}(image, keyChain)

		containerInfos := rc.controllerIndexers.GetContainerInfosForImage(image)
		rc.imageStore.UpdateContainerAssociations(image, containerInfos)
	}

	processingGroup.Wait()
}

func checkImageAvailability(log *logrus.Entry, imageName string, kc *keychain) (availMode store.AvailabilityMode) {
	ref, err := name.ParseReference(imageName)
	var parseErr *name.ErrBadName
	if errors.As(err, &parseErr) {
		log.WithField("availability_mode", store.BadImageName.String()).Error(err)
		return store.BadImageName
	} else if err != nil {
		log.WithField("availability_mode", store.UnknownError.String()).Error(err)
		return store.UnknownError
	}

	var imgErr error
	_ = wait.ExponentialBackoff(wait.Backoff{
		Duration: time.Second,
		Factor:   2,
		Steps:    2,
	}, func() (bool, error) {
		if kc != nil {
			_, imgErr = remote.Image(ref, remote.WithAuthFromKeychain(kc))
		} else {
			_, imgErr = remote.Image(ref)
		}

		availMode = store.Available
		if imgErr != nil {
			var transpErr *transport.Error
			errors.As(imgErr, &transpErr)

			var schemaErr *remote.ErrSchema1
			errors.As(imgErr, &schemaErr)

			if transpErr != nil {
				for _, transportError := range transpErr.Errors {
					if transportError.Code == transport.ManifestUnknownErrorCode {
						availMode = store.Absent
					}

					if transportError.Code == transport.UnauthorizedErrorCode {
						availMode = store.AuthnFailure
					}

					if transportError.Code == transport.DeniedErrorCode {
						availMode = store.AuthzFailure
					}
				}

				if availMode == store.Available {
					availMode = store.UnknownError
				}
			} else if schemaErr != nil {
				availMode = store.Available
			} else {
				availMode = store.UnknownError
			}
		}

		if availMode != store.Available {
			return false, nil
		}

		return true, nil
	})

	if availMode != store.Available {
		log.WithField("availability_mode", availMode.String()).Error(imgErr)
	}

	return
}
