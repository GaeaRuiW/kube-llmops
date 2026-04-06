package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/operator/internal/cli/printer"
	"github.com/kube-llmops/operator/internal/cli/util"
)

func newScaleCmd() *cobra.Command {
	var (
		replicas int32
		dryRun   bool
	)

	cmd := &cobra.Command{
		Use:   "scale <name>",
		Short: "Scale model replicas",
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
			md.Spec.Replicas = ptr.To(replicas)

			if dryRun {
				printer.PrintModelDeployments(os.Stdout, "yaml", []v1alpha1.ModelDeployment{md})
				return nil
			}

			if err := kc.CRClient.Update(ctx, &md); err != nil {
				return fmt.Errorf("failed to scale: %w", err)
			}
			fmt.Fprintf(os.Stdout, "ModelDeployment %q scaled to %d replicas\n", args[0], replicas)
			return nil
		},
	}

	cmd.Flags().Int32Var(&replicas, "replicas", 1, "Replica count")
	cmd.MarkFlagRequired("replicas")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print YAML without applying")

	return cmd
}
