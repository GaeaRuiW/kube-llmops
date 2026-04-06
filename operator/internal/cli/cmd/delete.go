package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/operator/internal/cli/util"
)

func newDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a model deployment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !force {
				fmt.Fprintf(os.Stdout, "Are you sure you want to delete ModelDeployment %q? [y/N]: ", name)
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
			ctx := context.Background()
			var md v1alpha1.ModelDeployment
			key := types.NamespacedName{Name: name, Namespace: kc.Namespace}
			if err := kc.CRClient.Get(ctx, key, &md); err != nil {
				return util.NotFoundError("ModelDeployment", name, kc.Namespace)
			}
			if err := kc.CRClient.Delete(ctx, &md); err != nil {
				return fmt.Errorf("failed to delete: %w", err)
			}
			fmt.Fprintf(os.Stdout, "ModelDeployment %q deleted.\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation")

	return cmd
}
