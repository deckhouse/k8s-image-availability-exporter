package registry

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func boolPtr(b bool) *bool {
	return &b
}

func TestShouldMonitorPod(t *testing.T) {
	cases := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{
			name: "unmanaged pod",
			pod:  &corev1.Pod{},
			want: true,
		},
		{
			name: "owned by ReplicaSet",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{
						Kind:       "ReplicaSet",
						Controller: boolPtr(true),
					}},
				},
			},
			want: false,
		},
		{
			name: "owned by Job",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{
						Kind:       "Job",
						Controller: boolPtr(true),
					}},
				},
			},
			want: false,
		},
		{
			name: "owned by DaemonSet",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{
						Kind:       "DaemonSet",
						Controller: boolPtr(true),
					}},
				},
			},
			want: false,
		},
		{
			name: "owned by StatefulSet",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{
						Kind:       "StatefulSet",
						Controller: boolPtr(true),
					}},
				},
			},
			want: false,
		},
		{
			name: "owned by custom controller",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{
						Kind:       "CustomController",
						Controller: boolPtr(true),
					}},
				},
			},
			want: true,
		},
		{
			name: "mixed owners",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "CustomController",
							Controller: boolPtr(true),
						},
						{
							Kind:       "ReplicaSet",
							Controller: boolPtr(true),
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, shouldMonitorPod(tc.pod))
		})
	}
}

func TestGetImagesFromPod(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "c1",
				Image: "img1",
			}},
			ImagePullSecrets:   []corev1.LocalObjectReference{{Name: "pull-secret"}},
			ServiceAccountName: "sa",
		},
	}

	obj, err := getImagesFromPod(pod)
	require.NoError(t, err)

	cis, ok := obj.(*controllerWithContainerInfos)
	require.True(t, ok)
	require.Equal(t, "Pod", cis.controllerKind)
	require.Equal(t, pod.Name, cis.Name)
	require.Equal(t, pod.Namespace, cis.Namespace)
	require.Equal(t, map[string]string{"c1": "img1"}, cis.containerToImages)
	require.Equal(t, pod.Spec.ImagePullSecrets, cis.pullSecretReferences)
	require.Equal(t, pod.Spec.ServiceAccountName, cis.serviceAccountName)
}
