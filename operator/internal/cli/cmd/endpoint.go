package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/operator/internal/cli/util"
)

func newEndpointCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "endpoint <name>",
		Short: "Print model API endpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			ctx := context.Background()
			var md v1alpha1.ModelDeployment
			if err := kc.CRClient.Get(ctx, types.NamespacedName{Name: args[0], Namespace: kc.Namespace}, &md); err != nil {
				return util.NotFoundError("ModelDeployment", args[0], kc.Namespace)
			}
			if md.Status.Endpoint == "" {
				return fmt.Errorf("ModelDeployment %q has no endpoint yet (phase: %s)", args[0], md.Status.Phase)
			}
			fmt.Fprintln(os.Stdout, md.Status.Endpoint)
			return nil
		},
	}
}
