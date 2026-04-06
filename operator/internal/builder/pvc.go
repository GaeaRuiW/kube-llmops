package builder

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
)

// getEngineCacheSize returns the default PVC storage size for a given engine.
func getEngineCacheSize(engine string) string {
	switch engine {
	case "tei":
		return "10Gi"
	case "llamacpp":
		return "30Gi"
	default:
		return "50Gi"
	}
}

// BuildPVC creates a PersistentVolumeClaim for caching model data.
func BuildPVC(md *v1alpha1.ModelDeployment, engine string) *corev1.PersistentVolumeClaim {
	labels := buildLabels(md.Name, engine)
	storageSize := getEngineCacheSize(engine)

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      md.Name + "-cache",
			Namespace: md.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(storageSize),
				},
			},
		},
	}
}
