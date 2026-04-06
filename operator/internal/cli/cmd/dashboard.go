package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/operator/internal/cli/util"
)

func newDashboardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dashboard",
		Short: "Open Grafana dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			ctx := context.Background()

			var lp v1alpha1.LLMPlatform
			if err := kc.CRClient.Get(ctx, types.NamespacedName{Name: "kube-llmops", Namespace: kc.Namespace}, &lp); err == nil {
				if lp.Spec.NodePort.Enabled && lp.Spec.NodePort.Host != "" {
					url := fmt.Sprintf("http://%s:30300", lp.Spec.NodePort.Host)
					fmt.Fprintln(os.Stdout, url)
					openBrowser(url)
					return nil
				}
				if lp.Status.Components.Grafana != nil && lp.Status.Components.Grafana.Endpoint != "" {
					fmt.Fprintln(os.Stdout, lp.Status.Components.Grafana.Endpoint)
					return nil
				}
			}

			info := util.ServiceMap["grafana"]
			url := fmt.Sprintf("http://localhost:%d", info.LocalPort)
			fmt.Fprintf(os.Stdout, "%s\n(Run 'kubectl llmops port-forward --service=grafana' first if not accessible)\n", url)
			return nil
		},
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		return
	}
	cmd.Start()
}
