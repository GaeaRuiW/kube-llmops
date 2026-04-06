package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/operator/internal/cli/printer"
	"github.com/kube-llmops/operator/internal/cli/util"
)

func newStatusCmd() *cobra.Command {
	var watch bool

	cmd := &cobra.Command{
		Use:   "status <name>",
		Short: "Show model deployment status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}

			for {
				ctx := context.Background()
				var md v1alpha1.ModelDeployment
				if err := kc.CRClient.Get(ctx, types.NamespacedName{Name: args[0], Namespace: kc.Namespace}, &md); err != nil {
					return util.NotFoundError("ModelDeployment", args[0], kc.Namespace)
				}

				printer.PrintModelDeployments(os.Stdout, output, []v1alpha1.ModelDeployment{md})

				if output == "table" || output == "wide" {
					fmt.Fprintln(os.Stdout)
					fmt.Fprintln(os.Stdout, "Conditions:")
					for _, c := range md.Status.Conditions {
						fmt.Fprintf(os.Stdout, "  %s: %s (%s)\n", c.Type, c.Status, c.Message)
					}
					if md.Status.Canary != nil {
						fmt.Fprintf(os.Stdout, "\nCanary: phase=%s endpoint=%s replicas=%d\n", md.Status.Canary.Phase, md.Status.Canary.Endpoint, md.Status.Canary.ReadyReplicas)
					}
				}

				if !watch {
					return nil
				}
				time.Sleep(2 * time.Second)
				fmt.Fprint(os.Stdout, "\033[2J\033[H")
			}
		},
	}

	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes")

	return cmd
}
