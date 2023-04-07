package registry_checker

import (
	"context"
	"fmt"

	"github.com/flant/k8s-image-availability-exporter/pkg/store"
	"github.com/google/go-containerregistry/pkg/authn"
	kubeauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	imageIndexName     = "image"
	labeledNSIndexName = "labeledNS"
)

type ControllerIndexers struct {
	namespaceIndexer      cache.Indexer
	serviceAccountIndexer cache.Indexer
	deploymentIndexer     cache.Indexer
	statefulSetIndexer    cache.Indexer
	daemonSetIndexer      cache.Indexer
	cronJobIndexer        cache.Indexer
	secretIndexer         cache.Indexer
}

type controllerWithContainerInfos struct {
	metav1.ObjectMeta
	controllerKind       string
	containerToImages    map[string]string
	pullSecretReferences []corev1.LocalObjectReference
	serviceAccountName   string
	enabled              bool
}

var (
	imageIndexers = cache.Indexers{
		imageIndexName: func(obj interface{}) (images []string, err error) {
			for _, v := range obj.(*controllerWithContainerInfos).containerToImages {
				images = append(images, v)
			}
			return
		},
	}
)

func (ci ControllerIndexers) validCi(cis *controllerWithContainerInfos) bool {
	if !cis.enabled {
		return false
	}

	nsList, _ := ci.namespaceIndexer.ByIndex(labeledNSIndexName, cis.Namespace)
	if len(nsList) == 0 {
		return false
	}

	return true
}

func namespaceIndexers(nsLabel string) cache.Indexers {
	return cache.Indexers{
		labeledNSIndexName: func(obj interface{}) ([]string, error) {
			ns := obj.(*corev1.Namespace)

			if len(nsLabel) == 0 {
				return []string{ns.GetName()}, nil
			}

			labels := ns.GetLabels()
			if len(labels) > 0 {
				if _, ok := labels[nsLabel]; ok {
					return []string{ns.GetName()}, nil
				}
			}

			return nil, nil
		},
	}
}

func getImagesFromDeployment(obj interface{}) (interface{}, error) {
	if cis, ok := obj.(*controllerWithContainerInfos); ok {
		return cis, nil
	}

	deployment := obj.(*appsv1.Deployment)

	deploymentCopy := deployment.DeepCopy()

	return &controllerWithContainerInfos{
		ObjectMeta:           deploymentCopy.ObjectMeta,
		controllerKind:       "Deployment",
		containerToImages:    extractImagesFromContainers(deploymentCopy.Spec.Template.Spec.Containers),
		pullSecretReferences: deploymentCopy.Spec.Template.Spec.ImagePullSecrets,
		serviceAccountName:   deploymentCopy.Spec.Template.Spec.ServiceAccountName,
		enabled:              *deploymentCopy.Spec.Replicas > 0,
	}, nil
}

func getImagesFromStatefulSet(obj interface{}) (interface{}, error) {
	if cis, ok := obj.(*controllerWithContainerInfos); ok {
		return cis, nil
	}

	statefulSet := obj.(*appsv1.StatefulSet)

	statefulSetCopy := statefulSet.DeepCopy()

	return &controllerWithContainerInfos{
		ObjectMeta:           statefulSetCopy.ObjectMeta,
		controllerKind:       "StatefulSet",
		containerToImages:    extractImagesFromContainers(statefulSetCopy.Spec.Template.Spec.Containers),
		pullSecretReferences: statefulSetCopy.Spec.Template.Spec.ImagePullSecrets,
		serviceAccountName:   statefulSetCopy.Spec.Template.Spec.ServiceAccountName,
		enabled:              *statefulSetCopy.Spec.Replicas > 0,
	}, nil
}

func getImagesFromDaemonSet(obj interface{}) (interface{}, error) {
	if cis, ok := obj.(*controllerWithContainerInfos); ok {
		return cis, nil
	}

	daemonSet := obj.(*appsv1.DaemonSet)

	daemonSetCopy := daemonSet.DeepCopy()

	return &controllerWithContainerInfos{
		ObjectMeta:           daemonSetCopy.ObjectMeta,
		controllerKind:       "DaemonSet",
		containerToImages:    extractImagesFromContainers(daemonSetCopy.Spec.Template.Spec.Containers),
		pullSecretReferences: daemonSetCopy.Spec.Template.Spec.ImagePullSecrets,
		serviceAccountName:   daemonSetCopy.Spec.Template.Spec.ServiceAccountName,
		enabled:              daemonSetCopy.Status.CurrentNumberScheduled > 0,
	}, nil
}

func getImagesFromCronJob(obj interface{}) (interface{}, error) {
	if cis, ok := obj.(*controllerWithContainerInfos); ok {
		return cis, nil
	}

	cronJob := obj.(*batchv1.CronJob)

	cronJobCopy := cronJob.DeepCopy()

	return &controllerWithContainerInfos{
		ObjectMeta:           cronJobCopy.ObjectMeta,
		controllerKind:       "CronJob",
		containerToImages:    extractImagesFromContainers(cronJobCopy.Spec.JobTemplate.Spec.Template.Spec.Containers),
		pullSecretReferences: cronJobCopy.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets,
		serviceAccountName:   cronJobCopy.Spec.JobTemplate.Spec.Template.Spec.ServiceAccountName,
		enabled:              !*cronJobCopy.Spec.Suspend,
	}, nil
}

func extractImagesFromContainers(containers []corev1.Container) map[string]string {
	ret := make(map[string]string)

	for _, container := range containers {
		ret[container.Name] = container.Image
	}

	return ret
}

func extractPullSecretKeysFromServiceAccount(namespace string, sa *corev1.ServiceAccount) (ret []string) {
	for _, ref := range sa.ImagePullSecrets {
		ret = append(ret, namespace+"/"+ref.Name)
	}

	return
}

func getCis(obj interface{}) *controllerWithContainerInfos {
	cis := obj.(*controllerWithContainerInfos)

	return cis
}

func (ci ControllerIndexers) ExtractPullSecretRefs(obj interface{}) (ret []string) {
	cis := obj.(*controllerWithContainerInfos)
	var pullSecretRefs []string
	for _, saRef := range cis.pullSecretReferences {
		pullSecretRefs = append(pullSecretRefs, fmt.Sprintf("%s/%s", cis.Namespace, saRef.Name))
	}

	// Image pull secret defined in Pod's `spec.ImagePullSecrets` takes preference over the secret from ServiceAccount.
	// We are acting the same way as kubelet does:
	// https://github.com/kubernetes/kubernetes/blob/88b31814f4a55c0af1c7d2712ce736a8fe08887e/plugin/pkg/admission/serviceaccount/admission.go#L163-L168.
	if len(pullSecretRefs) == 0 {
		var serviceAccountName string
		if len(cis.serviceAccountName) > 0 {
			serviceAccountName = cis.serviceAccountName
		} else {
			serviceAccountName = "default"
		}

		saRaw, exists, err := ci.serviceAccountIndexer.GetByKey(fmt.Sprintf("%s/%s", cis.Namespace, serviceAccountName))
		if err != nil {
			log.Warn(err)
			return
		}

		if exists {
			pullSecretRefs = append(pullSecretRefs, extractPullSecretKeysFromServiceAccount(cis.Namespace, saRaw.(*corev1.ServiceAccount))...)
		}
	}

	ret = append(ret, pullSecretRefs...)

	return
}

func (ci ControllerIndexers) GetObjectsByImageIndex(image string) (ret []interface{}) {
	for _, indexer := range []cache.Indexer{ci.deploymentIndexer, ci.statefulSetIndexer, ci.daemonSetIndexer, ci.cronJobIndexer} {
		objs, err := indexer.ByIndex(imageIndexName, image)
		if err != nil {
			panic(err)
		}

		ret = append(ret, objs...)
	}

	return
}

func (ci ControllerIndexers) GetContainerInfosForImage(image string) (ret []store.ContainerInfo) {
	objs := ci.GetObjectsByImageIndex(image)

	for _, obj := range objs {
		controllerWithInfos := obj.(*controllerWithContainerInfos)
		if !ci.validCi(controllerWithInfos) {
			continue
		}

		for k, v := range controllerWithInfos.containerToImages {
			if v != image {
				continue
			}

			ret = append(ret, store.ContainerInfo{
				Namespace:      controllerWithInfos.Namespace,
				ControllerKind: controllerWithInfos.controllerKind,
				ControllerName: controllerWithInfos.Name,
				Container:      k,
			})
		}
	}

	return
}

func (ci ControllerIndexers) GetKeychainForImage(image string) authn.Keychain {
	objs := ci.GetObjectsByImageIndex(image)

	var refSet = map[string]struct{}{}
	for _, obj := range objs {
		pullSecretRefs := ci.ExtractPullSecretRefs(obj)
		for _, ref := range pullSecretRefs {
			refSet[ref] = struct{}{}
		}
	}

	var dereferencedPullSecrets []corev1.Secret
	for ref := range refSet {
		secretObj, exists, err := ci.secretIndexer.GetByKey(ref)
		if err != nil {
			panic(err)
		}
		if !exists {
			continue
		}
		secretPtr := secretObj.(*corev1.Secret)
		dereferencedPullSecrets = append(dereferencedPullSecrets, *secretPtr)
	}

	if len(dereferencedPullSecrets) == 0 {
		return nil
	}

	kc, err := kubeauth.NewFromPullSecrets(context.TODO(), dereferencedPullSecrets)
	if err != nil {
		log.Panic(err)
	}

	return kc
}
