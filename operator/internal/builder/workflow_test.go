package builder

import (
	"testing"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildArgoWorkflow(t *testing.T) {
	ftr := &v1alpha1.FineTuneRun{
		ObjectMeta: metav1.ObjectMeta{Name: "gemma-lora-v1", Namespace: "default"},
		Spec: v1alpha1.FineTuneRunSpec{
			BaseModel:  "cyankiwi/gemma-4-26B-A4B-it-AWQ-4bit",
			OutputName: "gemma-4-lora-v1",
			Method:     "lora",
			DataSource: v1alpha1.DataSourceSpec{Type: "minio", Path: "s3://datasets/my-data/"},
			Training:   v1alpha1.TrainingSpec{Epochs: 3, BatchSize: 4, LearningRate: "2e-4", LoraRank: 16, LoraAlpha: 32},
			Resources:  v1alpha1.ModelResources{GPU: 1, Memory: "24Gi", CPU: "4"},
		},
	}
	wf := BuildArgoWorkflow(ftr, "kube-llmops")

	// --- Verify kind and apiVersion ---
	if wf.GetKind() != "Workflow" {
		t.Errorf("expected kind Workflow, got %q", wf.GetKind())
	}
	if wf.GetAPIVersion() != "argoproj.io/v1alpha1" {
		t.Errorf("expected apiVersion argoproj.io/v1alpha1, got %q", wf.GetAPIVersion())
	}

	// --- Verify name (ftr.Name + "-" + first-8-chars of OutputName, trailing hyphens trimmed) ---
	// "gemma-4-lora-v1"[:8] = "gemma-4-" → trimmed → "gemma-4"
	expectedName := "gemma-lora-v1-gemma-4"
	if wf.GetName() != expectedName {
		t.Errorf("expected name %q, got %q", expectedName, wf.GetName())
	}

	// --- Verify namespace ---
	if wf.GetNamespace() != "default" {
		t.Errorf("expected namespace 'default', got %q", wf.GetNamespace())
	}

	// --- Verify spec.templates has >= 7 entries ---
	spec, ok := wf.Object["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("spec is not a map")
	}
	templates, ok := spec["templates"].([]interface{})
	if !ok {
		t.Fatal("templates is not a slice")
	}
	if len(templates) < 7 {
		t.Errorf("expected at least 7 templates, got %d", len(templates))
	}

	// --- Verify DAG tasks exist ---
	mainTmpl, ok := templates[0].(map[string]interface{})
	if !ok {
		t.Fatal("main template is not a map")
	}
	if mainTmpl["name"] != "main" {
		t.Fatalf("expected first template to be 'main', got %q", mainTmpl["name"])
	}
	dag, ok := mainTmpl["dag"].(map[string]interface{})
	if !ok {
		t.Fatal("dag is not a map")
	}
	tasks, ok := dag["tasks"].([]interface{})
	if !ok {
		t.Fatal("tasks is not a slice")
	}

	taskNames := make(map[string]bool)
	for _, task := range tasks {
		taskMap := task.(map[string]interface{})
		taskNames[taskMap["name"].(string)] = true
	}

	for _, expected := range []string{"prepare-data", "finetune", "merge-upload", "evaluate", "quality-gate", "deploy"} {
		if !taskNames[expected] {
			t.Errorf("expected DAG task %q not found", expected)
		}
	}

	// --- Verify entrypoint ---
	if spec["entrypoint"] != "main" {
		t.Errorf("expected entrypoint 'main', got %v", spec["entrypoint"])
	}

	// --- Verify serviceAccountName ---
	if spec["serviceAccountName"] != "kube-llmops-finetune" {
		t.Errorf("expected serviceAccountName 'kube-llmops-finetune', got %v", spec["serviceAccountName"])
	}

	// --- Verify activeDeadlineSeconds ---
	if spec["activeDeadlineSeconds"] != int64(21600) {
		t.Errorf("expected activeDeadlineSeconds 21600, got %v", spec["activeDeadlineSeconds"])
	}

	// --- Verify volumeClaimTemplates ---
	vcts, ok := spec["volumeClaimTemplates"].([]interface{})
	if !ok {
		t.Fatal("volumeClaimTemplates is not a slice")
	}
	if len(vcts) != 1 {
		t.Fatalf("expected 1 volumeClaimTemplate, got %d", len(vcts))
	}
	vctMeta := vcts[0].(map[string]interface{})["metadata"].(map[string]interface{})
	if vctMeta["name"] != "workspace" {
		t.Errorf("expected VCT name 'workspace', got %v", vctMeta["name"])
	}
}

func TestBuildArgoWorkflow_Labels(t *testing.T) {
	ftr := &v1alpha1.FineTuneRun{
		ObjectMeta: metav1.ObjectMeta{Name: "gemma-lora-v1", Namespace: "default"},
		Spec: v1alpha1.FineTuneRunSpec{
			BaseModel:  "cyankiwi/gemma-4-26B-A4B-it-AWQ-4bit",
			OutputName: "gemma-4-lora-v1",
			DataSource: v1alpha1.DataSourceSpec{Type: "minio"},
		},
	}
	wf := BuildArgoWorkflow(ftr, "kube-llmops")

	labels := wf.GetLabels()
	expectedLabels := map[string]string{
		"app.kubernetes.io/name":    "finetune",
		"app.kubernetes.io/part-of": "kube-llmops",
		"kube-llmops/finetunerun":   "gemma-lora-v1",
	}
	for k, v := range expectedLabels {
		if labels[k] != v {
			t.Errorf("expected label %s=%s, got %q", k, v, labels[k])
		}
	}
}

func TestBuildArgoWorkflow_GPUResources(t *testing.T) {
	ftr := &v1alpha1.FineTuneRun{
		ObjectMeta: metav1.ObjectMeta{Name: "test-run", Namespace: "ml"},
		Spec: v1alpha1.FineTuneRunSpec{
			BaseModel:  "meta-llama/Llama-3-8b",
			OutputName: "llama3-ft",
			DataSource: v1alpha1.DataSourceSpec{Type: "minio"},
			Resources:  v1alpha1.ModelResources{GPU: 2, Memory: "48Gi", CPU: "8"},
		},
	}
	wf := BuildArgoWorkflow(ftr, "my-release")

	// --- Verify namespace propagation ---
	if wf.GetNamespace() != "ml" {
		t.Errorf("expected namespace 'ml', got %q", wf.GetNamespace())
	}

	spec := wf.Object["spec"].(map[string]interface{})
	templates := spec["templates"].([]interface{})

	// Find finetune template
	var finetuneContainer map[string]interface{}
	for _, tmpl := range templates {
		tmplMap := tmpl.(map[string]interface{})
		if tmplMap["name"] == "finetune" {
			finetuneContainer = tmplMap["container"].(map[string]interface{})
			break
		}
	}
	if finetuneContainer == nil {
		t.Fatal("finetune template not found")
	}

	resources, ok := finetuneContainer["resources"].(map[string]interface{})
	if !ok {
		t.Fatal("resources not found in finetune container")
	}
	limits, ok := resources["limits"].(map[string]interface{})
	if !ok {
		t.Fatal("limits not found in resources")
	}

	if limits["nvidia.com/gpu"] != "2" {
		t.Errorf("expected GPU limit '2', got %v", limits["nvidia.com/gpu"])
	}
	if limits["memory"] != "48Gi" {
		t.Errorf("expected memory limit '48Gi', got %v", limits["memory"])
	}
	if limits["cpu"] != "8" {
		t.Errorf("expected cpu limit '8', got %v", limits["cpu"])
	}

	requests, ok := resources["requests"].(map[string]interface{})
	if !ok {
		t.Fatal("requests not found in resources")
	}
	if requests["memory"] != "48Gi" {
		t.Errorf("expected memory request '48Gi', got %v", requests["memory"])
	}
	if requests["cpu"] != "8" {
		t.Errorf("expected cpu request '8', got %v", requests["cpu"])
	}

	// --- Verify MLflow env vars reference my-release ---
	envList := finetuneContainer["env"].([]interface{})
	envMap := make(map[string]string)
	for _, e := range envList {
		ev := e.(map[string]interface{})
		envMap[ev["name"].(string)] = ev["value"].(string)
	}
	if envMap["MLFLOW_TRACKING_URI"] != "http://my-release-mlflow:5000" {
		t.Errorf("expected MLFLOW_TRACKING_URI 'http://my-release-mlflow:5000', got %q", envMap["MLFLOW_TRACKING_URI"])
	}

	// --- Verify serviceAccountName uses releaseName ---
	if spec["serviceAccountName"] != "my-release-finetune" {
		t.Errorf("expected serviceAccountName 'my-release-finetune', got %v", spec["serviceAccountName"])
	}
}

func TestBuildArgoWorkflow_ShortOutputName(t *testing.T) {
	ftr := &v1alpha1.FineTuneRun{
		ObjectMeta: metav1.ObjectMeta{Name: "run", Namespace: "ns"},
		Spec: v1alpha1.FineTuneRunSpec{
			BaseModel:  "model",
			OutputName: "short",
			DataSource: v1alpha1.DataSourceSpec{Type: "pvc"},
		},
	}
	wf := BuildArgoWorkflow(ftr, "rel")

	// OutputName "short" (5 chars < 8) should be used in full.
	if wf.GetName() != "run-short" {
		t.Errorf("expected name 'run-short', got %q", wf.GetName())
	}
}

func TestBuildArgoWorkflow_NoGPU(t *testing.T) {
	ftr := &v1alpha1.FineTuneRun{
		ObjectMeta: metav1.ObjectMeta{Name: "cpu-run", Namespace: "default"},
		Spec: v1alpha1.FineTuneRunSpec{
			BaseModel:  "model",
			OutputName: "cpu-model-out",
			DataSource: v1alpha1.DataSourceSpec{Type: "huggingface"},
			Resources:  v1alpha1.ModelResources{}, // no resources
		},
	}
	wf := BuildArgoWorkflow(ftr, "rel")

	spec := wf.Object["spec"].(map[string]interface{})
	templates := spec["templates"].([]interface{})

	for _, tmpl := range templates {
		tmplMap := tmpl.(map[string]interface{})
		if tmplMap["name"] == "finetune" {
			container := tmplMap["container"].(map[string]interface{})
			if _, exists := container["resources"]; exists {
				t.Error("expected no resources on finetune container when ModelResources is empty")
			}
			return
		}
	}
	t.Fatal("finetune template not found")
}

func TestBuildArgoWorkflow_DAGDependencies(t *testing.T) {
	ftr := &v1alpha1.FineTuneRun{
		ObjectMeta: metav1.ObjectMeta{Name: "dep-test", Namespace: "default"},
		Spec: v1alpha1.FineTuneRunSpec{
			BaseModel:  "model",
			OutputName: "out12345678",
			DataSource: v1alpha1.DataSourceSpec{Type: "minio"},
		},
	}
	wf := BuildArgoWorkflow(ftr, "r")

	spec := wf.Object["spec"].(map[string]interface{})
	templates := spec["templates"].([]interface{})
	mainTmpl := templates[0].(map[string]interface{})
	dag := mainTmpl["dag"].(map[string]interface{})
	tasks := dag["tasks"].([]interface{})

	// Build dependency map: task-name → list of dependency names
	depMap := make(map[string][]string)
	for _, task := range tasks {
		taskMap := task.(map[string]interface{})
		name := taskMap["name"].(string)
		if deps, ok := taskMap["dependencies"].([]interface{}); ok {
			for _, d := range deps {
				depMap[name] = append(depMap[name], d.(string))
			}
		}
	}

	// prepare-data has no dependencies
	if len(depMap["prepare-data"]) != 0 {
		t.Errorf("prepare-data should have no deps, got %v", depMap["prepare-data"])
	}

	expectedDeps := map[string]string{
		"finetune":     "prepare-data",
		"merge-upload": "finetune",
		"evaluate":     "merge-upload",
		"quality-gate": "evaluate",
		"deploy":       "quality-gate",
	}
	for task, expectedDep := range expectedDeps {
		deps := depMap[task]
		if len(deps) != 1 || deps[0] != expectedDep {
			t.Errorf("task %q: expected dependency [%s], got %v", task, expectedDep, deps)
		}
	}
}
