package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/operator/internal/cli/printer"
	"github.com/kube-llmops/operator/internal/cli/util"
	"github.com/kube-llmops/operator/internal/engine"
)

func newDeployCmd() *cobra.Command {
	var (
		name          string
		engineFlag    string
		replicas      int32
		gpu           int32
		memory        string
		cpu           string
		accelerator   string
		engineArgs    []string
		prefixCaching bool
		dryRun        bool
	)

	cmd := &cobra.Command{
		Use:   "deploy <source>",
		Short: "Deploy a model",
		Long:  "Create a ModelDeployment CR from a HuggingFace model ID.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source := args[0]
			md := buildModelDeployment(source, name, engineFlag, replicas, gpu, memory, cpu, accelerator, engineArgs, prefixCaching)
			md.Namespace = namespace

			if dryRun {
				printer.PrintModelDeployments(os.Stdout, "yaml", []v1alpha1.ModelDeployment{*md})
				return nil
			}

			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			md.Namespace = kc.Namespace

			ctx := context.Background()
			if err := kc.CRClient.Create(ctx, md); err != nil {
				if errors.IsNotFound(err) {
					return fmt.Errorf("ModelDeployment CRD not found. Install the operator first:\n  helm install kube-llmops-operator charts/kube-llmops-operator/")
				}
				if errors.IsAlreadyExists(err) {
					return fmt.Errorf("ModelDeployment %q already exists. Use 'kubectl llmops scale' to update replicas or 'kubectl llmops canary' for model updates", md.Name)
				}
				return fmt.Errorf("failed to create ModelDeployment: %w", err)
			}

			resolvedEngine := engine.ResolveEngine(source, engineFlag)
			fmt.Fprintf(os.Stdout, "ModelDeployment %q created (engine: %s)\n", md.Name, resolvedEngine)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Deployment name (default: derived from source)")
	cmd.Flags().StringVar(&engineFlag, "engine", "auto", "Engine: auto|vllm|tei|llamacpp")
	cmd.Flags().Int32Var(&replicas, "replicas", 1, "Replica count")
	cmd.Flags().Int32Var(&gpu, "gpu", 1, "GPU count")
	cmd.Flags().StringVar(&memory, "memory", "16Gi", "Memory limit")
	cmd.Flags().StringVar(&cpu, "cpu", "4", "CPU limit")
	cmd.Flags().StringVar(&accelerator, "accelerator", "nvidia", "GPU vendor: nvidia|amd|gaudi")
	cmd.Flags().StringArrayVar(&engineArgs, "engine-arg", nil, "Extra engine args as KEY=VALUE (repeatable)")
	cmd.Flags().BoolVar(&prefixCaching, "prefix-caching", false, "Enable vLLM prefix caching")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print YAML without applying")

	return cmd
}

func buildModelDeployment(source, name, eng string, replicas, gpu int32, memory, cpu, accelerator string, engineArgsList []string, prefixCaching bool) *v1alpha1.ModelDeployment {
	if name == "" {
		name = util.SlugFromSource(source)
	}
	md := &v1alpha1.ModelDeployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "llmops.kubellmops.io/v1alpha1", Kind: "ModelDeployment"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.ModelDeploymentSpec{
			Source:        source,
			Engine:        eng,
			Replicas:      ptr.To(replicas),
			Resources:     v1alpha1.ModelResources{GPU: gpu, Memory: memory, CPU: cpu},
			Accelerator:   accelerator,
			PrefixCaching: prefixCaching,
		},
	}
	if len(engineArgsList) > 0 {
		md.Spec.EngineArgs = parseEngineArgs(engineArgsList)
	}
	return md
}

func parseEngineArgs(args []string) map[string]string {
	m := make(map[string]string, len(args))
	for _, a := range args {
		k, v, _ := strings.Cut(a, "=")
		m[k] = v
	}
	return m
}
