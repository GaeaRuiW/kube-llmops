package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: migrate <helm-release-name> [namespace]\n")
		os.Exit(1)
	}
	releaseName := os.Args[1]
	namespace := "default"
	if len(os.Args) > 2 {
		namespace = os.Args[2]
	}

	// Get Helm values
	out, err := exec.Command("helm", "get", "values", releaseName, "-n", namespace, "-o", "json").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get Helm values: %v\n", err)
		os.Exit(1)
	}

	var values map[string]interface{}
	if err := json.Unmarshal(out, &values); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse values: %v\n", err)
		os.Exit(1)
	}

	os.MkdirAll("generated", 0o755)

	// Generate LLMPlatform CR
	platform := generatePlatform(values, releaseName)
	writeYAML("generated/llmplatform.yaml", platform)
	fmt.Println("Generated: generated/llmplatform.yaml")

	// Generate ModelDeployment CRs
	global, _ := values["global"].(map[string]interface{})
	if global != nil {
		models, _ := global["models"].([]interface{})
		for _, m := range models {
			model, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			md := generateModelDeployment(model)
			name, _ := model["name"].(string)
			filename := fmt.Sprintf("generated/modeldeployment_%s.yaml", strings.ReplaceAll(name, "-", "_"))
			writeYAML(filename, md)
			fmt.Printf("Generated: %s\n", filename)
		}
	}

	fmt.Println("\nReview the generated CRs, then:")
	fmt.Printf("  helm uninstall %s -n %s\n", releaseName, namespace)
	fmt.Println("  kubectl apply -f generated/")
}

func generatePlatform(values map[string]interface{}, name string) map[string]interface{} {
	platform := map[string]interface{}{
		"apiVersion": "llmops.kubellmops.io/v1alpha1",
		"kind":       "LLMPlatform",
		"metadata":   map[string]interface{}{"name": name},
		"spec":       map[string]interface{}{},
	}

	spec := platform["spec"].(map[string]interface{})

	if litellm, ok := values["litellm"].(map[string]interface{}); ok {
		spec["gateway"] = map[string]interface{}{
			"enabled": litellm["enabled"],
		}
	}

	if obs, ok := values["observability"].(map[string]interface{}); ok {
		spec["observability"] = map[string]interface{}{
			"enabled": obs["enabled"],
		}
	}

	if global, ok := values["global"].(map[string]interface{}); ok {
		if modules, ok := global["modules"].(map[string]interface{}); ok {
			spec["modules"] = modules
		}
		if ms, ok := global["modelStore"].(map[string]interface{}); ok {
			spec["modelStore"] = ms
		}
		if np, ok := global["nodePort"].(map[string]interface{}); ok {
			spec["nodePort"] = np
		}
	}

	return platform
}

func generateModelDeployment(model map[string]interface{}) map[string]interface{} {
	md := map[string]interface{}{
		"apiVersion": "llmops.kubellmops.io/v1alpha1",
		"kind":       "ModelDeployment",
		"metadata":   map[string]interface{}{"name": model["name"]},
		"spec": map[string]interface{}{
			"source": model["source"],
		},
	}
	spec := md["spec"].(map[string]interface{})

	if replicas, ok := model["replicas"]; ok {
		spec["replicas"] = replicas
	}
	if resources, ok := model["resources"]; ok {
		spec["resources"] = resources
	}
	if engineArgs, ok := model["engineArgs"]; ok {
		spec["engineArgs"] = engineArgs
	}
	if engine, ok := model["engine"]; ok {
		spec["engine"] = engine
	}

	return md
}

func writeYAML(path string, data map[string]interface{}) {
	out, _ := yaml.Marshal(data)
	os.WriteFile(path, out, 0o644)
}
