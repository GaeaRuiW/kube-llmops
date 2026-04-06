package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/operator/internal/cli/util"
	"github.com/kube-llmops/operator/internal/engine"
)

func newTestCmd() *cobra.Command {
	var (
		prompt string
		stream bool
	)
	cmd := &cobra.Command{
		Use:   "test <name>",
		Short: "Send a test request to a model",
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

			modelType := engine.ResolveModelType(md.Spec.Source)
			if modelType != "llm" {
				return fmt.Errorf("model %q is an %s model and does not support chat", args[0], modelType)
			}

			gatewayInfo := util.ServiceMap["gateway"]
			gatewayURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", gatewayInfo.Name, kc.Namespace, gatewayInfo.Port)

			var lp v1alpha1.LLMPlatform
			if err := kc.CRClient.Get(ctx, types.NamespacedName{Name: "kube-llmops", Namespace: kc.Namespace}, &lp); err == nil {
				if lp.Spec.NodePort.Enabled && lp.Spec.NodePort.Host != "" {
					gatewayURL = fmt.Sprintf("http://%s:30400", lp.Spec.NodePort.Host)
				}
			}

			body := map[string]interface{}{
				"model": args[0],
				"messages": []map[string]string{
					{"role": "user", "content": prompt},
				},
				"stream": stream,
			}
			jsonBody, _ := json.Marshal(body)

			client := &http.Client{Timeout: 60 * time.Second}
			req, _ := http.NewRequestWithContext(ctx, "POST", gatewayURL+"/v1/chat/completions", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("LiteLLM gateway is not reachable at %s. Check 'kubectl llmops platform status'", gatewayURL)
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				respBody, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("gateway returned %d: %s", resp.StatusCode, string(respBody))
			}

			if stream {
				scanner := bufio.NewScanner(resp.Body)
				for scanner.Scan() {
					line := scanner.Text()
					if !strings.HasPrefix(line, "data: ") {
						continue
					}
					data := strings.TrimPrefix(line, "data: ")
					if data == "[DONE]" {
						fmt.Fprintln(os.Stdout)
						break
					}
					var chunk map[string]interface{}
					if err := json.Unmarshal([]byte(data), &chunk); err != nil {
						continue
					}
					if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
						if choice, ok := choices[0].(map[string]interface{}); ok {
							if delta, ok := choice["delta"].(map[string]interface{}); ok {
								if content, ok := delta["content"].(string); ok {
									fmt.Fprint(os.Stdout, content)
								}
							}
						}
					}
				}
				return nil
			}

			respBody, _ := io.ReadAll(resp.Body)
			var result map[string]interface{}
			json.Unmarshal(respBody, &result)

			if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if msg, ok := choice["message"].(map[string]interface{}); ok {
						fmt.Fprintf(os.Stdout, "%s\n", msg["content"])
						return nil
					}
				}
			}
			fmt.Fprintf(os.Stdout, "%s\n", string(respBody))
			return nil
		},
	}
	cmd.Flags().StringVar(&prompt, "prompt", "Hello", "Test prompt")
	cmd.Flags().BoolVar(&stream, "stream", false, "Stream response tokens")
	return cmd
}
