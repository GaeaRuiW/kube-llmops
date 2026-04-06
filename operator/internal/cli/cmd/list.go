package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/operator/internal/cli/printer"
	"github.com/kube-llmops/operator/internal/cli/util"
)

func newListCmd() *cobra.Command {
	var (
		allNamespaces bool
		watch         bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List model deployments",
		RunE: func(cmd *cobra.Command, args []string) error {
			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}

			for {
				ctx := context.Background()
				var mdList v1alpha1.ModelDeploymentList
				opts := []client.ListOption{}
				if !allNamespaces {
					opts = append(opts, client.InNamespace(kc.Namespace))
				}
				if err := kc.CRClient.List(ctx, &mdList, opts...); err != nil {
					return fmt.Errorf("failed to list ModelDeployments: %w", err)
				}
				if len(mdList.Items) == 0 {
					fmt.Fprintln(os.Stdout, "No model deployments found.")
				} else {
					printer.PrintModelDeployments(os.Stdout, output, mdList.Items)
				}
				if !watch {
					return nil
				}
				time.Sleep(2 * time.Second)
				fmt.Fprint(os.Stdout, "\033[2J\033[H") // clear screen
			}
		},
	}

	cmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "List across all namespaces")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes")

	return cmd
}
