package registry

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_namespaceIndexers(t *testing.T) {
	const nsLabel = "example.com/enabled"

	tests := []struct {
		name    string
		nsLabel string
		labels  map[string]string
		indexed bool
	}{
		{name: "no namespace label configured", nsLabel: "", labels: nil, indexed: true},
		{name: "no labels", nsLabel: nsLabel, labels: nil, indexed: false},
		{name: "other label", nsLabel: nsLabel, labels: map[string]string{"app": "test"}, indexed: false},
		{name: "empty value", nsLabel: nsLabel, labels: map[string]string{nsLabel: ""}, indexed: true},
		{name: "true", nsLabel: nsLabel, labels: map[string]string{nsLabel: "true"}, indexed: true},
		{name: "arbitrary value", nsLabel: nsLabel, labels: map[string]string{nsLabel: "yes"}, indexed: true},
		{name: "false", nsLabel: nsLabel, labels: map[string]string{nsLabel: "false"}, indexed: false},
		{name: "False", nsLabel: nsLabel, labels: map[string]string{nsLabel: "False"}, indexed: false},
		{name: "zero", nsLabel: nsLabel, labels: map[string]string{nsLabel: "0"}, indexed: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test", Labels: tt.labels}}
			got, err := namespaceIndexers(tt.nsLabel)[labeledNSIndexName](ns)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if indexed := len(got) != 0; indexed != tt.indexed {
				t.Errorf("namespace with labels %v indexed = %v, want %v", tt.labels, indexed, tt.indexed)
			}
		})
	}
}
