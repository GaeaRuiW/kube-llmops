package cmd

import (
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes/scheme"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
)

var (
	namespace  string
	output     string
	kubeconfig string
	kubeCtx    string
)

func init() {
	_ = v1alpha1.AddToScheme(scheme.Scheme)
}

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "kubectl-llmops",
		Short: "Manage LLM infrastructure on Kubernetes",
		Long:  "kubectl-llmops is a kubectl plugin for deploying, managing, and monitoring LLM models via the kube-llmops operator.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace (default: current context)")
	root.PersistentFlags().StringVarP(&output, "output", "o", "table", "Output format: table|json|yaml|wide")
	root.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig")
	root.PersistentFlags().StringVar(&kubeCtx, "context", "", "Kubernetes context to use")

	// Model lifecycle commands
	root.AddCommand(newDeployCmd())
	root.AddCommand(newListCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newScaleCmd())
	root.AddCommand(newDeleteCmd())

	// Canary commands
	root.AddCommand(newCanaryCmd())
	root.AddCommand(newPromoteCmd())
	root.AddCommand(newRollbackCmd())

	// DX commands
	root.AddCommand(newLogsCmd())
	root.AddCommand(newTestCmd())
	root.AddCommand(newEndpointCmd())
	root.AddCommand(newPortForwardCmd())
	root.AddCommand(newDashboardCmd())

	// Subcommand groups
	root.AddCommand(newFinetuneCmd())
	root.AddCommand(newPlatformCmd())
	root.AddCommand(newRAGCmd())

	// Migration
	root.AddCommand(newMigrateCmd())

	return root
}
