package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"github.com/kube-llmops/operator/internal/cli/util"
)

func newPortForwardCmd() *cobra.Command {
	var service string
	cmd := &cobra.Command{
		Use:   "port-forward",
		Short: "Forward a local port to a platform service",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, ok := util.ServiceMap[service]
			if !ok {
				return fmt.Errorf("unknown service %q. Valid: gateway, grafana, langfuse, dify, minio", service)
			}

			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			ctx := context.Background()

			endpoints, err := kc.Clientset.CoreV1().Endpoints(kc.Namespace).Get(ctx, info.Name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("service %q not found in namespace %q", info.Name, kc.Namespace)
			}
			var podName string
			for _, subset := range endpoints.Subsets {
				for _, addr := range subset.Addresses {
					if addr.TargetRef != nil && addr.TargetRef.Kind == "Pod" {
						podName = addr.TargetRef.Name
						break
					}
				}
				if podName != "" {
					break
				}
			}
			if podName == "" {
				return fmt.Errorf("no ready pods found for service %q", info.Name)
			}

			stopChan := make(chan struct{}, 1)
			readyChan := make(chan struct{})
			portMapping := fmt.Sprintf("%d:%d", info.LocalPort, info.Port)

			restClient := kc.Clientset.CoreV1().RESTClient()
			req := restClient.Post().
				Resource("pods").
				Namespace(kc.Namespace).
				Name(podName).
				SubResource("portforward")

			transport, upgrader, err := spdy.RoundTripperFor(kc.RestConfig)
			if err != nil {
				return fmt.Errorf("failed to create transport: %w", err)
			}
			dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())

			fw, err := portforward.New(dialer, []string{portMapping}, stopChan, readyChan, os.Stdout, os.Stderr)
			if err != nil {
				return fmt.Errorf("failed to create port-forward: %w", err)
			}

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt)
			go func() {
				<-sigChan
				close(stopChan)
			}()

			fmt.Fprintf(os.Stdout, "Forwarding %s:%d → localhost:%d\n", info.Name, info.Port, info.LocalPort)
			return fw.ForwardPorts()
		},
	}
	cmd.Flags().StringVar(&service, "service", "gateway", "Service: gateway|grafana|langfuse|dify|minio")
	return cmd
}
