package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newMigrateCmd() *cobra.Command {
	var outputDir string
	cmd := &cobra.Command{
		Use:   "migrate <helm-release>",
		Short: "Migrate from Helm release to operator CRs",
		Long:  "Reads Helm release values and generates LLMPlatform + ModelDeployment CRs.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			releaseName := args[0]
			ns := namespace
			if ns == "" {
				ns = "default"
			}

			out, err := exec.Command("helm", "get", "values", releaseName, "-n", ns, "-o", "json").Output()
			if err != nil {
				return fmt.Errorf("failed to get Helm values for release %q: %w", releaseName, err)
			}

			var values map[string]interface{}
			if err := json.Unmarshal(out, &values); err != nil {
				return fmt.Errorf("failed to parse Helm values: %w", err)
			}

			os.MkdirAll(outputDir, 0o755)

			platform := migratePlatform(values, releaseName)
			writeYAMLFile(fmt.Sprintf("%s/llmplatform.yaml", outputDir), platform)
			fmt.Fprintf(os.Stdout, "Generated: %s/llmplatform.yaml\n", outputDir)

			global, _ := values["global"].(map[string]interface{})
			if global != nil {
				models, _ := global["models"].([]interface{})
				for _, m := range models {
					model, ok := m.(map[string]interface{})
					if !ok {
						continue
					}
					md := migrateModelDeployment(model)
					name, _ := model["name"].(string)
					filename := fmt.Sprintf("%s/modeldeployment_%s.yaml", outputDir, strings.ReplaceAll(name, "-", "_"))
					writeYAMLFile(filename, md)
					fmt.Fprintf(os.Stdout, "Generated: %s\n", filename)
				}
			}

			fmt.Fprintln(os.Stdout, "\nReview the generated CRs, then:")
			fmt.Fprintf(os.Stdout, "  helm uninstall %s -n %s\n", releaseName, ns)
			fmt.Fprintf(os.Stdout, "  kubectl apply -f %s/\n", outputDir)
			return nil
		},
	}
	cmd.Flags().StringVar(&outputDir, "output-dir", "./generated", "Output directory for generated CRs")
	return cmd
}

func migratePlatform(values map[string]interface{}, name string) map[string]interface{} {
	platform := map[string]interface{}{
		"apiVersion": "llmops.kubellmops.io/v1alpha1",
		"kind":       "LLMPlatform",
		"metadata":   map[string]interface{}{"name": name},
		"spec":       map[string]interface{}{},
	}
	spec := platform["spec"].(map[string]interface{})

	if litellm, ok := values["litellm"].(map[string]interface{}); ok {
		spec["gateway"] = map[string]interface{}{"enabled": litellm["enabled"]}
	}
	if obs, ok := values["observability"].(map[string]interface{}); ok {
		spec["observability"] = map[string]interface{}{"enabled": obs["enabled"]}
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

func migrateModelDeployment(model map[string]interface{}) map[string]interface{} {
	md := map[string]interface{}{
		"apiVersion": "llmops.kubellmops.io/v1alpha1",
		"kind":       "ModelDeployment",
		"metadata":   map[string]interface{}{"name": model["name"]},
		"spec":       map[string]interface{}{"source": model["source"]},
	}
	spec := md["spec"].(map[string]interface{})
	for _, key := range []string{"replicas", "resources", "engineArgs", "engine"} {
		if v, ok := model[key]; ok {
			spec[key] = v
		}
	}
	return md
}

func writeYAMLFile(path string, data map[string]interface{}) {
	out, _ := yaml.Marshal(data)
	os.WriteFile(path, out, 0o644)
}
