package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/operator/internal/cli/printer"
	"github.com/kube-llmops/operator/internal/cli/util"
)

func newCanaryCmd() *cobra.Command {
	var (
		target string
		weight int32
		dryRun bool
	)
	cmd := &cobra.Command{
		Use:   "canary <name>",
		Short: "Configure canary deployment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			ctx := context.Background()
			var md v1alpha1.ModelDeployment
			key := types.NamespacedName{Name: args[0], Namespace: kc.Namespace}
			if err := kc.CRClient.Get(ctx, key, &md); err != nil {
				return util.NotFoundError("ModelDeployment", args[0], kc.Namespace)
			}
			applyCanary(&md, target, weight)
			if dryRun {
				printer.PrintModelDeployments(os.Stdout, "yaml", []v1alpha1.ModelDeployment{md})
				return nil
			}
			if err := kc.CRClient.Update(ctx, &md); err != nil {
				return fmt.Errorf("failed to update canary: %w", err)
			}
			fmt.Fprintf(os.Stdout, "Canary configured: target=%s weight=%d%%\n", target, weight)
			return nil
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "Canary model source (required)")
	cmd.Flags().Int32Var(&weight, "weight", 0, "Traffic weight 0-100 (required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print YAML without applying")
	cmd.MarkFlagRequired("target")
	cmd.MarkFlagRequired("weight")
	return cmd
}

func newPromoteCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "promote <name>",
		Short: "Promote canary to primary",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			ctx := context.Background()
			var md v1alpha1.ModelDeployment
			key := types.NamespacedName{Name: args[0], Namespace: kc.Namespace}
			if err := kc.CRClient.Get(ctx, key, &md); err != nil {
				return util.NotFoundError("ModelDeployment", args[0], kc.Namespace)
			}
			if md.Spec.Canary == nil {
				return fmt.Errorf("ModelDeployment %q has no canary configured", args[0])
			}
			newSource := md.Spec.Canary.Source
			applyPromote(&md)
			if dryRun {
				printer.PrintModelDeployments(os.Stdout, "yaml", []v1alpha1.ModelDeployment{md})
				return nil
			}
			if err := kc.CRClient.Update(ctx, &md); err != nil {
				return fmt.Errorf("failed to promote: %w", err)
			}
			fmt.Fprintf(os.Stdout, "Promoted: source updated to %q, canary removed\n", newSource)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print YAML without applying")
	return cmd
}

func newRollbackCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "rollback <name>",
		Short: "Remove canary, keep primary",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			ctx := context.Background()
			var md v1alpha1.ModelDeployment
			key := types.NamespacedName{Name: args[0], Namespace: kc.Namespace}
			if err := kc.CRClient.Get(ctx, key, &md); err != nil {
				return util.NotFoundError("ModelDeployment", args[0], kc.Namespace)
			}
			if md.Spec.Canary == nil {
				return fmt.Errorf("ModelDeployment %q has no canary configured", args[0])
			}
			applyRollback(&md)
			if dryRun {
				printer.PrintModelDeployments(os.Stdout, "yaml", []v1alpha1.ModelDeployment{md})
				return nil
			}
			if err := kc.CRClient.Update(ctx, &md); err != nil {
				return fmt.Errorf("failed to rollback: %w", err)
			}
			fmt.Fprintf(os.Stdout, "Rolled back: canary removed, source unchanged (%q)\n", md.Spec.Source)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print YAML without applying")
	return cmd
}

func applyCanary(md *v1alpha1.ModelDeployment, target string, weight int32) {
	md.Spec.Canary = &v1alpha1.CanaryConfig{Source: target, Weight: weight}
}

func applyPromote(md *v1alpha1.ModelDeployment) {
	md.Spec.Source = md.Spec.Canary.Source
	md.Spec.Canary = nil
}

func applyRollback(md *v1alpha1.ModelDeployment) {
	md.Spec.Canary = nil
}
