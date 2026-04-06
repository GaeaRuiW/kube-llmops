package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kube-llmops/operator/internal/cli/util"
)

func newLogsCmd() *cobra.Command {
	var (
		follow    bool
		tail      int64
		container string
	)
	cmd := &cobra.Command{
		Use:   "logs <name>",
		Short: "View model pod logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			ctx := context.Background()

			pods, err := kc.Clientset.CoreV1().Pods(kc.Namespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("kube-llmops/model=%s", args[0]),
			})
			if err != nil {
				return fmt.Errorf("failed to list pods: %w", err)
			}
			if len(pods.Items) == 0 {
				return util.NotFoundError("pods for model", args[0], kc.Namespace)
			}

			podName := pods.Items[0].Name
			opts := &corev1.PodLogOptions{
				Container: container,
				Follow:    follow,
				TailLines: &tail,
			}

			req := kc.Clientset.CoreV1().Pods(kc.Namespace).GetLogs(podName, opts)
			stream, err := req.Stream(ctx)
			if err != nil {
				return fmt.Errorf("failed to stream logs: %w", err)
			}
			defer stream.Close()
			io.Copy(os.Stdout, stream)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Stream logs")
	cmd.Flags().Int64Var(&tail, "tail", 100, "Lines to show")
	cmd.Flags().StringVar(&container, "container", "model-server", "Container name")

	return cmd
}
