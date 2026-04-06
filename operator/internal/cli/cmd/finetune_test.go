package cmd

import (
	"testing"
)

func TestBuildFineTuneRun_Basic(t *testing.T) {
	ftr := buildFineTuneRun("org/model", "model-lora-v1", "lora", "s3://data/train.json", "alpaca", 3, 4, "2e-4", 16, 32, 1, false, false, false, 0)
	if ftr.Spec.BaseModel != "org/model" {
		t.Errorf("expected baseModel %q, got %q", "org/model", ftr.Spec.BaseModel)
	}
	if ftr.Spec.OutputName != "model-lora-v1" {
		t.Errorf("expected outputName %q, got %q", "model-lora-v1", ftr.Spec.OutputName)
	}
	if ftr.Spec.Method != "lora" {
		t.Errorf("expected method lora, got %q", ftr.Spec.Method)
	}
	if ftr.Spec.DataSource.Type != "minio" {
		t.Errorf("expected datasource type minio, got %q", ftr.Spec.DataSource.Type)
	}
	if ftr.Spec.Training.Epochs != 3 {
		t.Errorf("expected 3 epochs, got %d", ftr.Spec.Training.Epochs)
	}
	if ftr.Spec.Training.LoraRank != 16 {
		t.Errorf("expected loraRank 16, got %d", ftr.Spec.Training.LoraRank)
	}
}

func TestBuildFineTuneRun_WithEvalAndDeploy(t *testing.T) {
	ftr := buildFineTuneRun("org/model", "out", "qlora", "s3://d/t.json", "alpaca", 1, 2, "1e-4", 8, 16, 1, true, true, true, 20)
	if !ftr.Spec.Evaluation.Enabled {
		t.Error("expected evaluation enabled")
	}
	if !ftr.Spec.QualityGate.Enabled {
		t.Error("expected quality gate enabled")
	}
	if !ftr.Spec.Deploy.Enabled {
		t.Error("expected deploy enabled")
	}
	if ftr.Spec.Deploy.CanaryWeight != 20 {
		t.Errorf("expected canary weight 20, got %d", ftr.Spec.Deploy.CanaryWeight)
	}
}

func TestInferDataSourceType(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"s3://bucket/path", "minio"},
		{"minio://bucket/path", "minio"},
		{"hf://dataset/name", "huggingface"},
		{"/data/train.json", "pvc"},
		{"dataset-name", "huggingface"},
	}
	for _, tt := range tests {
		got := inferDataSourceType(tt.path)
		if got != tt.want {
			t.Errorf("inferDataSourceType(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
