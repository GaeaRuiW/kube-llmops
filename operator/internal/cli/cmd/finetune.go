package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/operator/internal/cli/printer"
	"github.com/kube-llmops/operator/internal/cli/util"
)

func newFinetuneCmd() *cobra.Command {
	ft := &cobra.Command{
		Use:   "finetune",
		Short: "Manage fine-tuning runs",
	}
	ft.AddCommand(newFinetuneCreateCmd())
	ft.AddCommand(newFinetuneListCmd())
	ft.AddCommand(newFinetuneStatusCmd())
	ft.AddCommand(newFinetuneLogsCmd())
	ft.AddCommand(newFinetuneDeleteCmd())
	return ft
}

func newFinetuneCreateCmd() *cobra.Command {
	var (
		baseModel    string
		outputName   string
		method       string
		dataSource   string
		dataFormat   string
		epochs       int32
		batchSize    int32
		learningRate string
		loraRank     int32
		loraAlpha    int32
		gpu          int32
		eval         bool
		qualityGate  bool
		autoDeploy   bool
		canaryWeight int32
		dryRun       bool
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a fine-tuning run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if qualityGate && !eval {
				return fmt.Errorf("--quality-gate requires --eval")
			}
			ftr := buildFineTuneRun(baseModel, outputName, method, dataSource, dataFormat, epochs, batchSize, learningRate, loraRank, loraAlpha, gpu, eval, qualityGate, autoDeploy, canaryWeight)

			if dryRun {
				printer.PrintFineTuneRuns(os.Stdout, "yaml", []v1alpha1.FineTuneRun{*ftr})
				return nil
			}

			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			ftr.Namespace = kc.Namespace
			if err := kc.CRClient.Create(context.Background(), ftr); err != nil {
				return fmt.Errorf("failed to create FineTuneRun: %w", err)
			}
			fmt.Fprintf(os.Stdout, "FineTuneRun %q created\n", ftr.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&baseModel, "base-model", "", "Base model source (required)")
	cmd.Flags().StringVar(&outputName, "output-name", "", "Output model name (required)")
	cmd.Flags().StringVar(&method, "method", "lora", "Training method: lora|qlora|full")
	cmd.Flags().StringVar(&dataSource, "data-source", "", "Data path (required)")
	cmd.Flags().StringVar(&dataFormat, "data-format", "alpaca", "Data format: alpaca|sharegpt")
	cmd.Flags().Int32Var(&epochs, "epochs", 3, "Training epochs")
	cmd.Flags().Int32Var(&batchSize, "batch-size", 4, "Batch size")
	cmd.Flags().StringVar(&learningRate, "learning-rate", "2e-4", "Learning rate")
	cmd.Flags().Int32Var(&loraRank, "lora-rank", 16, "LoRA rank")
	cmd.Flags().Int32Var(&loraAlpha, "lora-alpha", 32, "LoRA alpha")
	cmd.Flags().Int32Var(&gpu, "gpu", 1, "GPU count")
	cmd.Flags().BoolVar(&eval, "eval", false, "Enable evaluation")
	cmd.Flags().BoolVar(&qualityGate, "quality-gate", false, "Enable quality gate")
	cmd.Flags().BoolVar(&autoDeploy, "auto-deploy", false, "Auto-deploy on success")
	cmd.Flags().Int32Var(&canaryWeight, "canary-weight", 0, "Canary weight for auto-deploy")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print YAML without applying")
	cmd.MarkFlagRequired("base-model")
	cmd.MarkFlagRequired("output-name")
	cmd.MarkFlagRequired("data-source")
	return cmd
}

func newFinetuneListCmd() *cobra.Command {
	var allNamespaces bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List fine-tuning runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			var list v1alpha1.FineTuneRunList
			opts := []client.ListOption{}
			if !allNamespaces {
				opts = append(opts, client.InNamespace(kc.Namespace))
			}
			if err := kc.CRClient.List(context.Background(), &list, opts...); err != nil {
				return fmt.Errorf("failed to list FineTuneRuns: %w", err)
			}
			if len(list.Items) == 0 {
				fmt.Fprintln(os.Stdout, "No fine-tuning runs found.")
				return nil
			}
			printer.PrintFineTuneRuns(os.Stdout, output, list.Items)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "List across all namespaces")
	return cmd
}

func newFinetuneStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <name>",
		Short: "Show fine-tuning run status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			var ftr v1alpha1.FineTuneRun
			if err := kc.CRClient.Get(context.Background(), types.NamespacedName{Name: args[0], Namespace: kc.Namespace}, &ftr); err != nil {
				return util.NotFoundError("FineTuneRun", args[0], kc.Namespace)
			}
			printer.PrintFineTuneRuns(os.Stdout, output, []v1alpha1.FineTuneRun{ftr})
			if output == "table" {
				if ftr.Status.MLflow.RunID != "" {
					fmt.Fprintf(os.Stdout, "\nMLflow: run=%s experiment=%s\n", ftr.Status.MLflow.RunID, ftr.Status.MLflow.ExperimentName)
				}
				if ftr.Status.QualityGate.Message != "" {
					fmt.Fprintf(os.Stdout, "Quality Gate: passed=%v %s\n", ftr.Status.QualityGate.Passed, ftr.Status.QualityGate.Message)
				}
			}
			return nil
		},
	}
}

func newFinetuneLogsCmd() *cobra.Command {
	var (
		follow bool
		step   string
	)
	cmd := &cobra.Command{
		Use:   "logs <name>",
		Short: "View fine-tuning run logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			ctx := context.Background()

			var ftr v1alpha1.FineTuneRun
			if err := kc.CRClient.Get(ctx, types.NamespacedName{Name: args[0], Namespace: kc.Namespace}, &ftr); err != nil {
				return util.NotFoundError("FineTuneRun", args[0], kc.Namespace)
			}

			if ftr.Status.ArgoWorkflow == "" {
				return fmt.Errorf("FineTuneRun %q has no Argo Workflow yet (phase: %s)", args[0], ftr.Status.Phase)
			}

			labelSelector := fmt.Sprintf("workflows.argoproj.io/workflow=%s", ftr.Status.ArgoWorkflow)
			if step != "" {
				labelSelector += fmt.Sprintf(",workflows.argoproj.io/node-name=%s.%s", ftr.Status.ArgoWorkflow, step)
			}

			pods, err := kc.Clientset.CoreV1().Pods(kc.Namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
			if err != nil || len(pods.Items) == 0 {
				return fmt.Errorf("no workflow pods found for %q", args[0])
			}

			podName := pods.Items[len(pods.Items)-1].Name
			var tail int64 = 100
			stream, err := kc.Clientset.CoreV1().Pods(kc.Namespace).GetLogs(podName, &corev1.PodLogOptions{
				Container: "main",
				Follow:    follow,
				TailLines: &tail,
			}).Stream(ctx)
			if err != nil {
				return fmt.Errorf("failed to stream logs: %w", err)
			}
			defer stream.Close()
			io.Copy(os.Stdout, stream)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Stream logs")
	cmd.Flags().StringVar(&step, "step", "", "DAG step: prepare-data|finetune|merge-upload|evaluate|quality-gate|deploy")
	return cmd
}

func newFinetuneDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a fine-tuning run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !force {
				fmt.Fprintf(os.Stdout, "Are you sure you want to delete FineTuneRun %q? [y/N]: ", name)
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(answer)), "y") {
					fmt.Fprintln(os.Stdout, "Cancelled.")
					return nil
				}
			}
			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			var ftr v1alpha1.FineTuneRun
			if err := kc.CRClient.Get(context.Background(), types.NamespacedName{Name: name, Namespace: kc.Namespace}, &ftr); err != nil {
				return util.NotFoundError("FineTuneRun", name, kc.Namespace)
			}
			if err := kc.CRClient.Delete(context.Background(), &ftr); err != nil {
				return fmt.Errorf("failed to delete: %w", err)
			}
			fmt.Fprintf(os.Stdout, "FineTuneRun %q deleted.\n", name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation")
	return cmd
}

func buildFineTuneRun(baseModel, outputName, method, dataSource, dataFormat string, epochs, batchSize int32, learningRate string, loraRank, loraAlpha, gpu int32, eval, qualityGate, autoDeploy bool, canaryWeight int32) *v1alpha1.FineTuneRun {
	ftr := &v1alpha1.FineTuneRun{
		TypeMeta:   metav1.TypeMeta{APIVersion: "llmops.kubellmops.io/v1alpha1", Kind: "FineTuneRun"},
		ObjectMeta: metav1.ObjectMeta{Name: outputName},
		Spec: v1alpha1.FineTuneRunSpec{
			BaseModel:  baseModel,
			OutputName: outputName,
			Method:     method,
			DataSource: v1alpha1.DataSourceSpec{
				Type:   inferDataSourceType(dataSource),
				Path:   dataSource,
				Format: dataFormat,
			},
			Training: v1alpha1.TrainingSpec{
				Epochs:       epochs,
				BatchSize:    batchSize,
				LearningRate: learningRate,
				LoraRank:     loraRank,
				LoraAlpha:    loraAlpha,
			},
			Resources:   v1alpha1.ModelResources{GPU: gpu, Memory: "16Gi", CPU: "4"},
			Evaluation:  v1alpha1.EvaluationSpec{Enabled: eval},
			QualityGate: v1alpha1.QualityGateSpec{Enabled: qualityGate},
			Deploy:      v1alpha1.DeploySpec{Enabled: autoDeploy, CanaryWeight: canaryWeight},
		},
	}
	return ftr
}

func inferDataSourceType(path string) string {
	if strings.HasPrefix(path, "s3://") || strings.HasPrefix(path, "minio://") {
		return "minio"
	}
	if strings.HasPrefix(path, "/") {
		return "pvc"
	}
	if strings.HasPrefix(path, "hf://") {
		return "huggingface"
	}
	return "huggingface"
}
