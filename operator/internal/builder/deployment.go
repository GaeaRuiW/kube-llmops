package builder

import (
	"fmt"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
)

// engineConfig holds per-engine defaults.
type engineConfig struct {
	Image string
	Port  int32
}

var engines = map[string]engineConfig{
	"vllm":     {Image: "vllm/vllm-openai:v0.19.0", Port: 8000},
	"tei":      {Image: "ghcr.io/huggingface/text-embeddings-inference:cpu-1.9.3", Port: 8080},
	"llamacpp": {Image: "ghcr.io/ggml-org/llama.cpp:server-b8672", Port: 8080},
}

// acceleratorResources maps accelerator vendor to the Kubernetes device plugin resource name.
var acceleratorResources = map[string]string{
	"nvidia": "nvidia.com/gpu",
	"amd":    "amd.com/gpu",
	"gaudi":  "habana.ai/gaudi",
}

// modelSlug converts a HuggingFace model source (e.g. "Qwen/Qwen2.5-7B-Instruct")
// to a filesystem-safe slug by replacing "/" with "--".
func modelSlug(source string) string {
	return strings.ReplaceAll(source, "/", "--")
}

// BuildDeployment creates a Kubernetes Deployment for the given ModelDeployment CR.
func BuildDeployment(md *v1alpha1.ModelDeployment, engine string, platform *v1alpha1.LLMPlatform) *appsv1.Deployment {
	cfg := engines[engine]
	slug := modelSlug(md.Spec.Source)

	labels := map[string]string{
		"app.kubernetes.io/name":      engine,
		"app.kubernetes.io/instance":  md.Name,
		"app.kubernetes.io/part-of":   "kube-llmops",
		"app.kubernetes.io/component": "model-serving",
		"kube-llmops/model":           md.Name,
		"kube-llmops/engine":          engine,
	}

	// --- Engine arguments ---
	args := buildEngineArgs(md, engine, slug, cfg.Port)

	// --- GPU resource name ---
	gpuResourceName := resolveGPUResource(md)

	// --- Volumes & volume mounts ---
	volumes, volumeMounts := buildVolumes(engine, md.Name)

	// --- Container resources ---
	containerResources := buildContainerResources(md, gpuResourceName)

	// --- Probes ---
	healthPath := "/health"
	probe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: healthPath,
				Port: intstr.FromInt32(cfg.Port),
			},
		},
		InitialDelaySeconds: 30,
		PeriodSeconds:       10,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}

	// --- Main container ---
	mainContainer := corev1.Container{
		Name:           "model-server",
		Image:          cfg.Image,
		Args:           args,
		Ports:          []corev1.ContainerPort{{Name: "http", ContainerPort: cfg.Port, Protocol: corev1.ProtocolTCP}},
		Resources:      containerResources,
		VolumeMounts:   volumeMounts,
		ReadinessProbe: probe,
		LivenessProbe:  probe.DeepCopy(),
	}

	// --- Init container (model-loader) ---
	initContainers := buildInitContainers(md, platform, slug)

	// --- Tolerations ---
	tolerations := buildTolerations(gpuResourceName)

	// --- Strategy ---
	strategy := appsv1.DeploymentStrategy{
		Type: appsv1.RecreateDeploymentStrategyType,
	}

	var terminationGracePeriod int64 = 90
	enableServiceLinks := false

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      md.Name,
			Namespace: md.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: md.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/instance": md.Name,
				},
			},
			Strategy: strategy,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					InitContainers:                initContainers,
					Containers:                    []corev1.Container{mainContainer},
					Volumes:                       volumes,
					Tolerations:                   tolerations,
					EnableServiceLinks:            &enableServiceLinks,
					TerminationGracePeriodSeconds: &terminationGracePeriod,
				},
			},
		},
	}

	return dep
}

// buildEngineArgs constructs the CLI arguments for the inference engine.
func buildEngineArgs(md *v1alpha1.ModelDeployment, engine, slug string, port int32) []string {
	modelPath := fmt.Sprintf("/models/%s", slug)
	var args []string

	switch engine {
	case "vllm":
		args = []string{
			"--model", modelPath,
			"--served-model-name", md.Name,
			"--host", "0.0.0.0",
			"--port", fmt.Sprintf("%d", port),
		}
		if md.Spec.PrefixCaching {
			args = append(args, "--enable-prefix-caching")
		}
	case "tei":
		args = []string{
			"--model-id", modelPath,
			"--hostname", "0.0.0.0",
			"--port", fmt.Sprintf("%d", port),
		}
	case "llamacpp":
		args = []string{
			"--model", modelPath,
			"--host", "0.0.0.0",
			"--port", fmt.Sprintf("%d", port),
		}
	}

	// Append user-specified engine args in sorted key order for deterministic output.
	if len(md.Spec.EngineArgs) > 0 {
		keys := make([]string, 0, len(md.Spec.EngineArgs))
		for k := range md.Spec.EngineArgs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			args = append(args, k, md.Spec.EngineArgs[k])
		}
	}

	return args
}

// resolveGPUResource returns the Kubernetes device plugin resource name.
// If MIGDevice is set, it is used directly; otherwise the accelerator default is used.
func resolveGPUResource(md *v1alpha1.ModelDeployment) string {
	if md.Spec.MIGDevice != "" {
		return md.Spec.MIGDevice
	}
	if res, ok := acceleratorResources[md.Spec.Accelerator]; ok {
		return res
	}
	return "nvidia.com/gpu"
}

// buildVolumes returns the volumes and volume mounts for the deployment.
func buildVolumes(engine, mdName string) ([]corev1.Volume, []corev1.VolumeMount) {
	volumes := []corev1.Volume{
		{
			Name: "model-cache",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: fmt.Sprintf("%s-cache", mdName),
				},
			},
		},
	}
	mounts := []corev1.VolumeMount{
		{Name: "model-cache", MountPath: "/models"},
	}

	if engine == "vllm" {
		shmSize := resource.MustParse("8Gi")
		volumes = append(volumes, corev1.Volume{
			Name: "dshm",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium:    corev1.StorageMediumMemory,
					SizeLimit: &shmSize,
				},
			},
		})
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "dshm",
			MountPath: "/dev/shm",
		})
	}

	return volumes, mounts
}

// buildContainerResources constructs the resource requirements for the main container.
func buildContainerResources(md *v1alpha1.ModelDeployment, gpuResourceName string) corev1.ResourceRequirements {
	limits := corev1.ResourceList{}
	requests := corev1.ResourceList{}

	if md.Spec.Resources.Memory != "" {
		mem := resource.MustParse(md.Spec.Resources.Memory)
		limits[corev1.ResourceMemory] = mem
		requests[corev1.ResourceMemory] = mem
	}
	if md.Spec.Resources.CPU != "" {
		cpu := resource.MustParse(md.Spec.Resources.CPU)
		limits[corev1.ResourceCPU] = cpu
		requests[corev1.ResourceCPU] = cpu
	}
	if md.Spec.Resources.GPU > 0 {
		gpuQty := resource.MustParse(fmt.Sprintf("%d", md.Spec.Resources.GPU))
		limits[corev1.ResourceName(gpuResourceName)] = gpuQty
		requests[corev1.ResourceName(gpuResourceName)] = gpuQty
	}

	return corev1.ResourceRequirements{
		Limits:   limits,
		Requests: requests,
	}
}

// buildInitContainers creates the model-loader init container using platform model store settings,
// with optional per-model overrides.
func buildInitContainers(md *v1alpha1.ModelDeployment, platform *v1alpha1.LLMPlatform, slug string) []corev1.Container {
	if platform == nil || !platform.Spec.ModelStore.Enabled {
		return nil
	}

	ms := platform.Spec.ModelStore

	// Apply per-model overrides.
	endpoint := ms.Endpoint
	bucket := ms.Bucket
	if md.Spec.ModelStore != nil {
		if md.Spec.ModelStore.Endpoint != "" {
			endpoint = md.Spec.ModelStore.Endpoint
		}
		if md.Spec.ModelStore.Bucket != "" {
			bucket = md.Spec.ModelStore.Bucket
		}
	}

	concurrency := ms.HFTransferConcurrency
	if concurrency == 0 {
		concurrency = 4
	}

	envVars := []corev1.EnvVar{
		{Name: "MODEL_SOURCE", Value: md.Spec.Source},
		{Name: "MODEL_SLUG", Value: slug},
		{Name: "MINIO_ENDPOINT", Value: endpoint},
		{Name: "MINIO_BUCKET", Value: bucket},
		{Name: "MINIO_ACCESS_KEY", Value: ms.AccessKey},
		{Name: "MINIO_SECRET_KEY", Value: ms.SecretKey},
		{Name: "HF_TRANSFER_CONCURRENCY", Value: fmt.Sprintf("%d", concurrency)},
		{Name: "HF_HUB_ENABLE_HF_TRANSFER", Value: "1"},
	}

	// Pass HF token for gated models (Llama, Gemma, etc.)
	if platform.Spec.HFToken != "" {
		envVars = append(envVars, corev1.EnvVar{Name: "HF_TOKEN", Value: platform.Spec.HFToken})
	}

	return []corev1.Container{
		{
			Name:         "model-loader",
			Image:        ms.Image,
			Env:          envVars,
			VolumeMounts: []corev1.VolumeMount{
				{Name: "model-cache", MountPath: "/models"},
			},
		},
	}
}

// buildTolerations creates GPU tolerations so pods can schedule on GPU nodes.
func buildTolerations(gpuResourceName string) []corev1.Toleration {
	return []corev1.Toleration{
		{
			Key:      gpuResourceName,
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}
}
