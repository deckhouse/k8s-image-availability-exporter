package registry_checker

import (
	"context"
	"fmt"
	"reflect"

	"github.com/flant/k8s-image-availability-exporter/pkg/store"
	credentialprovider "github.com/vdemeester/k8s-pkg-credentialprovider"
	credentialprovidersecrets "github.com/vdemeester/k8s-pkg-credentialprovider/secrets"
	appsv1 "k8s.io/api/apps/v1"
	batchv1beta "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	imageIndexName = "images"
)

var (
	deploymentIndexers = cache.Indexers{
		imageIndexName: GetImagesFromDeployment,
	}
	statefulSetIndexers = cache.Indexers{
		imageIndexName: GetImagesFromStatefulSet,
	}
	daemonSetIndexers = cache.Indexers{
		imageIndexName: GetImagesFromDaemonSet,
	}
	cronJobIndexers = cache.Indexers{
		imageIndexName: GetImagesFromCronJob,
	}
)

func GetImagesFromDeployment(obj interface{}) ([]string, error) {
	deployment := obj.(*appsv1.Deployment)

	return extractImagesFromContainers(deployment.Spec.Template.Spec.Containers), nil
}

func GetImagesFromStatefulSet(obj interface{}) ([]string, error) {
	statefulSet := obj.(*appsv1.StatefulSet)

	return extractImagesFromContainers(statefulSet.Spec.Template.Spec.Containers), nil
}

func GetImagesFromDaemonSet(obj interface{}) ([]string, error) {
	daemonSet := obj.(*appsv1.DaemonSet)

	return extractImagesFromContainers(daemonSet.Spec.Template.Spec.Containers), nil
}

func GetImagesFromCronJob(obj interface{}) ([]string, error) {
	cronJob := obj.(*batchv1beta.CronJob)

	return extractImagesFromContainers(cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers), nil
}

func extractContainerInfoFromContainers(image, namespace, controllerKind, controllerName string, containers []corev1.Container) (ret []store.ContainerInfo) {
	for _, container := range containers {
		if container.Image != image {
			continue
		}

		ret = append(ret, store.ContainerInfo{
			Namespace:      namespace,
			ControllerKind: controllerKind,
			ControllerName: controllerName,
			Container:      container.Name,
		})
	}

	return
}

func extractImagesFromContainers(containers []corev1.Container) (ret []string) {
	for _, container := range containers {
		ret = append(ret, container.Image)
	}

	return
}

func extractPullSecretRefsFromPodSpec(namespace string, spec corev1.PodSpec) (ret []string) {
	for _, ref := range spec.ImagePullSecrets {
		ret = append(ret, namespace+"/"+ref.Name)
	}

	return
}

func extractPullSecretRefsFromServiceAccount(namespace string, spec corev1.ServiceAccount) (ret []string) {
	for _, ref := range spec.ImagePullSecrets {
		ret = append(ret, namespace+"/"+ref.Name)
	}

	return
}

func ExtractImages(obj interface{}) ([]string, error) {
	switch typedObj := obj.(type) {
	case *appsv1.Deployment:
		return extractImagesFromContainers(typedObj.Spec.Template.Spec.Containers), nil
	case *appsv1.StatefulSet:
		return extractImagesFromContainers(typedObj.Spec.Template.Spec.Containers), nil
	case *appsv1.DaemonSet:
		return extractImagesFromContainers(typedObj.Spec.Template.Spec.Containers), nil
	case *batchv1beta.CronJob:
		return extractImagesFromContainers(typedObj.Spec.JobTemplate.Spec.Template.Spec.Containers), nil
	default:
		panic(fmt.Errorf("%q not of types *appsv1.Deployment, *appsv1.StatefulSet, *appsv1.DaemonSet, *batchv1beta.CronJob", reflect.TypeOf(typedObj)))
	}
}

func ExtractContainerInfos(image string, obj interface{}) []store.ContainerInfo {
	switch typedObj := obj.(type) {
	case *appsv1.Deployment:
		return extractContainerInfoFromContainers(image, typedObj.Namespace, "Deployment", typedObj.Name, typedObj.Spec.Template.Spec.Containers)
	case *appsv1.StatefulSet:
		return extractContainerInfoFromContainers(image, typedObj.Namespace, "StatefulSet", typedObj.Name, typedObj.Spec.Template.Spec.Containers)
	case *appsv1.DaemonSet:
		return extractContainerInfoFromContainers(image, typedObj.Namespace, "DaemonSet", typedObj.Name, typedObj.Spec.Template.Spec.Containers)
	case *batchv1beta.CronJob:
		return extractContainerInfoFromContainers(image, typedObj.Namespace, "CronJob", typedObj.Name, typedObj.Spec.JobTemplate.Spec.Template.Spec.Containers)
	default:
		panic(fmt.Errorf("%q not of types *appsv1.Deployment, *appsv1.StatefulSet, *appsv1.DaemonSet, *batchv1beta.CronJob", reflect.TypeOf(typedObj)))
	}
}

func ExtractPullSecretRefs(kubeClient *kubernetes.Clientset, obj interface{}) (ret []string) {
	var (
		namespace string
		podSpec   corev1.PodSpec
	)

	switch typedObj := obj.(type) {
	case *appsv1.Deployment:
		namespace = typedObj.Namespace
		podSpec = typedObj.Spec.Template.Spec
	case *appsv1.StatefulSet:
		namespace = typedObj.Namespace
		podSpec = typedObj.Spec.Template.Spec
	case *appsv1.DaemonSet:
		namespace = typedObj.Namespace
		podSpec = typedObj.Spec.Template.Spec
	case *batchv1beta.CronJob:
		namespace = typedObj.Namespace
		podSpec = typedObj.Spec.JobTemplate.Spec.Template.Spec
	default:
		panic(fmt.Errorf("%q not of types *appsv1.Deployment, *appsv1.StatefulSet, *appsv1.DaemonSet, *batchv1beta.CronJob", reflect.TypeOf(typedObj)))
	}

	pullSecretRefs := extractPullSecretRefsFromPodSpec(namespace, podSpec)
	// Image pull secret defined in Pod's `spec.ImagePullSecrets` takes preference over the secret from ServiceAccount.
	// We are acting the same way as kubelet does:
	// https://github.com/kubernetes/kubernetes/blob/88b31814f4a55c0af1c7d2712ce736a8fe08887e/plugin/pkg/admission/serviceaccount/admission.go#L163-L168.
	if len(pullSecretRefs) == 0 && len(podSpec.ServiceAccountName) > 0 {
		serviceAccount, err := kubeClient.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), podSpec.ServiceAccountName, metav1.GetOptions{})
		if err == nil {
			pullSecretRefs = append(pullSecretRefs, extractPullSecretRefsFromServiceAccount(namespace, *serviceAccount)...)
		}
	}
	ret = append(ret, pullSecretRefs...)

	return
}

func EqualObjects(a, b interface{}) bool {
	aObj := a.(metav1.Common)
	bObj := b.(metav1.Common)
	return aObj.GetResourceVersion() == bObj.GetResourceVersion()
}

type ControllerIndexers struct {
	deploymentIndexer  cache.Indexer
	statefulSetIndexer cache.Indexer
	daemonSetIndexer   cache.Indexer
	cronJobIndexer     cache.Indexer
	secretIndexer      cache.Indexer
}

func (ci ControllerIndexers) GetObjectsByIndex(image string) (ret []interface{}) {
	for _, indexer := range []cache.Indexer{ci.deploymentIndexer, ci.statefulSetIndexer, ci.daemonSetIndexer, ci.cronJobIndexer} {
		objs, err := indexer.ByIndex(imageIndexName, image)
		if err != nil {
			panic(err)
		}

		ret = append(ret, objs...)
	}

	return
}

func (ci ControllerIndexers) GetKeysByIndex(image string) (ret []string) {
	for _, indexer := range []cache.Indexer{ci.deploymentIndexer, ci.statefulSetIndexer, ci.daemonSetIndexer, ci.cronJobIndexer} {
		refs, err := indexer.IndexKeys(imageIndexName, image)
		if err != nil {
			panic(err)
		}

		ret = append(ret, refs...)
	}

	return
}

func (ci ControllerIndexers) CheckImageExistence(image string) bool {
	keys := ci.GetKeysByIndex(image)

	return len(keys) > 0
}

func (ci ControllerIndexers) GetContainerInfosForImage(image string) (ret []store.ContainerInfo) {
	objs := ci.GetObjectsByIndex(image)

	for _, obj := range objs {
		ret = append(ret, ExtractContainerInfos(image, obj)...)
	}

	return
}

func (ci ControllerIndexers) GetKeychainForImage(kubeClient *kubernetes.Clientset, image string) *keychain {
	objs := ci.GetObjectsByIndex(image)

	var refSet = map[string]struct{}{}
	for _, obj := range objs {
		pullSecretRefs := ExtractPullSecretRefs(kubeClient, obj)
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

	kr, err := credentialprovidersecrets.MakeDockerKeyring(dereferencedPullSecrets, credentialprovider.NewDockerKeyring())
	if err != nil {
		panic(err)
	}

	kc := &keychain{
		keyring: kr,
		size:    len(dereferencedPullSecrets),
	}

	return kc
}
