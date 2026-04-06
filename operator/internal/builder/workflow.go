package builder

import (
	"fmt"
	"strings"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// BuildArgoWorkflow creates an Argo Workflow (as *unstructured.Unstructured to
// avoid importing Argo CRD dependencies) for a FineTuneRun CR.
//
// The workflow contains a six-step DAG:
//
//	prepare-data → finetune → merge-upload → evaluate → quality-gate → deploy
func BuildArgoWorkflow(ftr *v1alpha1.FineTuneRun, releaseName string) *unstructured.Unstructured {
	// --- Name: ftr.Name + truncated OutputName (first 8 chars) ---
	outputTrunc := ftr.Spec.OutputName
	if len(outputTrunc) > 8 {
		outputTrunc = outputTrunc[:8]
	}
	name := strings.TrimRight(fmt.Sprintf("%s-%s", ftr.Name, outputTrunc), "-")

	mlflowURL := fmt.Sprintf("http://%s-mlflow:5000", releaseName)
	minioEndpoint := fmt.Sprintf("%s-minio:9000", releaseName)

	// --- Labels ---
	labels := map[string]interface{}{
		"app.kubernetes.io/name":    "finetune",
		"app.kubernetes.io/part-of": "kube-llmops",
		"kube-llmops/finetunerun":   ftr.Name,
	}

	// --- Shared volume mount ---
	workspaceMount := map[string]interface{}{
		"name":      "workspace",
		"mountPath": "/workspace",
	}

	// --- Task templates ---
	prepareData := map[string]interface{}{
		"name": "prepare-data",
		"container": map[string]interface{}{
			"image": "kube-llmops/model-loader:latest",
			"env": []interface{}{
				envVar("S3_ENDPOINT", minioEndpoint),
				envVar("MODEL_SOURCE", ftr.Spec.BaseModel),
			},
			"volumeMounts": []interface{}{workspaceMount},
		},
	}

	finetuneContainer := map[string]interface{}{
		"image": "hiyouga/llamafactory:latest",
		"env": []interface{}{
			envVar("MLFLOW_TRACKING_URI", mlflowURL),
			envVar("MLFLOW_EXPERIMENT_NAME", ftr.Spec.OutputName),
		},
		"volumeMounts": []interface{}{workspaceMount},
	}
	if res := buildGPUResources(ftr.Spec.Resources); len(res) > 0 {
		finetuneContainer["resources"] = res
	}
	finetune := map[string]interface{}{
		"name":      "finetune",
		"container": finetuneContainer,
	}

	mergeUpload := map[string]interface{}{
		"name": "merge-upload",
		"container": map[string]interface{}{
			"image": "kube-llmops/model-loader:latest",
			"env": []interface{}{
				envVar("MLFLOW_TRACKING_URI", mlflowURL),
				envVar("S3_ENDPOINT", minioEndpoint),
			},
			"volumeMounts": []interface{}{workspaceMount},
		},
	}

	evaluate := map[string]interface{}{
		"name": "evaluate",
		"container": map[string]interface{}{
			"image":        "python:3.13-slim",
			"volumeMounts": []interface{}{workspaceMount},
		},
	}

	qualityGate := map[string]interface{}{
		"name": "quality-gate",
		"container": map[string]interface{}{
			"image":        "python:3.13-slim",
			"volumeMounts": []interface{}{workspaceMount},
		},
	}

	deploy := map[string]interface{}{
		"name": "deploy",
		"container": map[string]interface{}{
			"image":        "bitnami/kubectl:latest",
			"volumeMounts": []interface{}{workspaceMount},
		},
	}

	// --- Main DAG template ---
	mainTemplate := map[string]interface{}{
		"name": "main",
		"dag": map[string]interface{}{
			"tasks": []interface{}{
				dagTask("prepare-data", "prepare-data", nil),
				dagTask("finetune", "finetune", []string{"prepare-data"}),
				dagTask("merge-upload", "merge-upload", []string{"finetune"}),
				dagTask("evaluate", "evaluate", []string{"merge-upload"}),
				dagTask("quality-gate", "quality-gate", []string{"evaluate"}),
				dagTask("deploy", "deploy", []string{"quality-gate"}),
			},
		},
	}

	// --- Volume claim templates ---
	volumeClaimTemplates := []interface{}{
		map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "workspace",
			},
			"spec": map[string]interface{}{
				"accessModes": []interface{}{"ReadWriteOnce"},
				"resources": map[string]interface{}{
					"requests": map[string]interface{}{
						"storage": "10Gi",
					},
				},
			},
		},
	}

	// --- Assemble the Workflow ---
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Workflow",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": ftr.Namespace,
				"labels":    labels,
			},
			"spec": map[string]interface{}{
				"serviceAccountName":  fmt.Sprintf("%s-finetune", releaseName),
				"activeDeadlineSeconds": int64(21600),
				"entrypoint":           "main",
				"volumeClaimTemplates": volumeClaimTemplates,
				"templates": []interface{}{
					mainTemplate,
					prepareData,
					finetune,
					mergeUpload,
					evaluate,
					qualityGate,
					deploy,
				},
			},
		},
	}
}

// envVar builds a single Kubernetes-style env var entry.
func envVar(name, value string) map[string]interface{} {
	return map[string]interface{}{"name": name, "value": value}
}

// dagTask builds a single DAG task entry.
func dagTask(name, template string, deps []string) map[string]interface{} {
	t := map[string]interface{}{
		"name":     name,
		"template": template,
	}
	if len(deps) > 0 {
		d := make([]interface{}, len(deps))
		for i, dep := range deps {
			d[i] = dep
		}
		t["dependencies"] = d
	}
	return t
}

// buildGPUResources constructs a container resources map from ModelResources.
func buildGPUResources(r v1alpha1.ModelResources) map[string]interface{} {
	limits := map[string]interface{}{}
	requests := map[string]interface{}{}

	if r.GPU > 0 {
		limits["nvidia.com/gpu"] = fmt.Sprintf("%d", r.GPU)
	}
	if r.Memory != "" {
		limits["memory"] = r.Memory
		requests["memory"] = r.Memory
	}
	if r.CPU != "" {
		limits["cpu"] = r.CPU
		requests["cpu"] = r.CPU
	}

	if len(limits) == 0 && len(requests) == 0 {
		return nil
	}

	res := map[string]interface{}{}
	if len(limits) > 0 {
		res["limits"] = limits
	}
	if len(requests) > 0 {
		res["requests"] = requests
	}
	return res
}
