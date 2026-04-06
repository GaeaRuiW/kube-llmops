package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/operator/internal/cli/util"
)

func newRAGCmd() *cobra.Command {
	r := &cobra.Command{
		Use:   "rag",
		Short: "Manage RAG knowledge bases",
	}
	r.AddCommand(newRAGListKBCmd())
	r.AddCommand(newRAGCreateKBCmd())
	r.AddCommand(newRAGUploadCmd())
	r.AddCommand(newRAGDeleteKBCmd())
	r.AddCommand(newRAGQueryCmd())
	r.AddCommand(newRAGEvalCmd())
	return r
}

func difyRequest(ctx context.Context, kc *util.KubeClients, method, path string, body io.Reader) (*http.Response, error) {
	endpoint, err := findDifyEndpoint(ctx, kc)
	if err != nil {
		return nil, err
	}
	apiKey, err := findDifyAPIKey(ctx, kc)
	if err != nil {
		return nil, err
	}
	url := endpoint + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if body != nil && method != "GET" {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	return client.Do(req)
}

func findDifyEndpoint(ctx context.Context, kc *util.KubeClients) (string, error) {
	var lp v1alpha1.LLMPlatform
	if err := kc.CRClient.Get(ctx, types.NamespacedName{Name: "kube-llmops", Namespace: kc.Namespace}, &lp); err == nil {
		if lp.Spec.NodePort.Enabled && lp.Spec.NodePort.Host != "" {
			return fmt.Sprintf("http://%s:30500", lp.Spec.NodePort.Host), nil
		}
	}
	svc, err := kc.Clientset.CoreV1().Services(kc.Namespace).Get(ctx, "kube-llmops-dify-api", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("Dify is not available. Enable RAG module: kubectl llmops platform update --enable rag")
	}
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", svc.Name, kc.Namespace, svc.Spec.Ports[0].Port), nil
}

func findDifyAPIKey(ctx context.Context, kc *util.KubeClients) (string, error) {
	secret, err := kc.Clientset.CoreV1().Secrets(kc.Namespace).Get(ctx, "kube-llmops-dify-credentials", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("Dify credentials secret not found")
	}
	key, ok := secret.Data["api-key"]
	if !ok {
		return "", fmt.Errorf("api-key not found in Dify credentials secret")
	}
	return string(key), nil
}

func newRAGListKBCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-kb",
		Short: "List knowledge bases",
		RunE: func(cmd *cobra.Command, args []string) error {
			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			ctx := context.Background()
			resp, err := difyRequest(ctx, kc, "GET", "/v1/datasets?page=1&limit=100", nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			if output == "json" {
				fmt.Fprintln(os.Stdout, string(body))
				return nil
			}

			var result map[string]interface{}
			json.Unmarshal(body, &result)
			data, _ := result["data"].([]interface{})
			fmt.Fprintf(os.Stdout, "%-36s  %-30s  %s\n", "ID", "NAME", "DOCS")
			for _, d := range data {
				kb, _ := d.(map[string]interface{})
				id, _ := kb["id"].(string)
				name, _ := kb["name"].(string)
				count, _ := kb["document_count"].(float64)
				fmt.Fprintf(os.Stdout, "%-36s  %-30s  %.0f\n", id, name, count)
			}
			return nil
		},
	}
}

func newRAGCreateKBCmd() *cobra.Command {
	var (
		description    string
		embeddingModel string
	)
	cmd := &cobra.Command{
		Use:   "create-kb <name>",
		Short: "Create a knowledge base",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			ctx := context.Background()
			payload := map[string]string{"name": args[0]}
			if description != "" {
				payload["description"] = description
			}
			if embeddingModel != "" {
				payload["embedding_model"] = embeddingModel
			}
			jsonBody, _ := json.Marshal(payload)
			resp, err := difyRequest(ctx, kc, "POST", "/v1/datasets", bytes.NewReader(jsonBody))
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode >= 400 {
				return fmt.Errorf("failed to create KB: %s", string(body))
			}
			var result map[string]interface{}
			json.Unmarshal(body, &result)
			fmt.Fprintf(os.Stdout, "Knowledge base %q created (id: %s)\n", args[0], result["id"])
			return nil
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "KB description")
	cmd.Flags().StringVar(&embeddingModel, "embedding-model", "", "Embedding model for KB (auto-discovers from LiteLLM if empty)")
	return cmd
}

func newRAGUploadCmd() *cobra.Command {
	var (
		chunkSize    int
		chunkOverlap int
	)
	cmd := &cobra.Command{
		Use:   "upload <kb-name> <file...>",
		Short: "Upload documents to a knowledge base",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			kbName := args[0]
			files := args[1:]

			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			ctx := context.Background()

			kbID, err := findKBIDByName(ctx, kc, kbName)
			if err != nil {
				return err
			}

			for _, filePath := range files {
				if err := uploadFile(ctx, kc, kbID, filePath, chunkSize, chunkOverlap); err != nil {
					return fmt.Errorf("failed to upload %s: %w", filePath, err)
				}
				fmt.Fprintf(os.Stdout, "Uploaded: %s\n", filePath)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&chunkSize, "chunk-size", 1000, "Chunk size for indexing")
	cmd.Flags().IntVar(&chunkOverlap, "chunk-overlap", 200, "Chunk overlap for indexing")
	return cmd
}

func findKBIDByName(ctx context.Context, kc *util.KubeClients, name string) (string, error) {
	resp, err := difyRequest(ctx, kc, "GET", "/v1/datasets?page=1&limit=100", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	data, _ := result["data"].([]interface{})
	for _, d := range data {
		kb, _ := d.(map[string]interface{})
		if kb["name"] == name {
			id, _ := kb["id"].(string)
			return id, nil
		}
	}
	return "", fmt.Errorf("knowledge base %q not found", name)
}

func uploadFile(ctx context.Context, kc *util.KubeClients, kbID, filePath string, chunkSize, chunkOverlap int) error {
	endpoint, err := findDifyEndpoint(ctx, kc)
	if err != nil {
		return err
	}
	apiKey, err := findDifyAPIKey(ctx, kc)
	if err != nil {
		return err
	}

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", filepath.Base(filePath))
	io.Copy(part, f)
	processRule := fmt.Sprintf(`{"indexing_technique":"high_quality","process_rule":{"mode":"custom","rules":{"segmentation":{"separator":"\n","max_tokens":%d,"chunk_overlap":%d}}}}`, chunkSize, chunkOverlap)
	writer.WriteField("data", processRule)
	writer.Close()

	url := fmt.Sprintf("%s/v1/datasets/%s/document/create_by_file", endpoint, kbID)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, &buf)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func newRAGDeleteKBCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete-kb <name>",
		Short: "Delete a knowledge base",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				fmt.Fprintf(os.Stdout, "Are you sure you want to delete knowledge base %q? [y/N]: ", args[0])
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

			kbID, err := findKBIDByName(ctx, kc, args[0])
			if err != nil {
				return err
			}

			delResp, err := difyRequest(ctx, kc, "DELETE", "/v1/datasets/"+kbID, nil)
			if err != nil {
				return err
			}
			delResp.Body.Close()
			fmt.Fprintf(os.Stdout, "Knowledge base %q deleted.\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation")
	return cmd
}

func newRAGQueryCmd() *cobra.Command {
	var (
		prompt string
		topK   int
	)
	cmd := &cobra.Command{
		Use:   "query <kb-name>",
		Short: "Query a knowledge base",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			ctx := context.Background()

			kbID, err := findKBIDByName(ctx, kc, args[0])
			if err != nil {
				return err
			}

			payload := map[string]interface{}{
				"query": prompt,
				"top_k": topK,
			}
			jsonBody, _ := json.Marshal(payload)
			queryResp, err := difyRequest(ctx, kc, "POST", "/v1/datasets/"+kbID+"/retrieve", bytes.NewReader(jsonBody))
			if err != nil {
				return err
			}
			defer queryResp.Body.Close()
			queryBody, _ := io.ReadAll(queryResp.Body)

			if output == "json" {
				fmt.Fprintln(os.Stdout, string(queryBody))
				return nil
			}

			var queryResult map[string]interface{}
			json.Unmarshal(queryBody, &queryResult)
			records, _ := queryResult["records"].([]interface{})
			for i, r := range records {
				rec, _ := r.(map[string]interface{})
				content, _ := rec["content"].(string)
				score, _ := rec["score"].(float64)
				fmt.Fprintf(os.Stdout, "[%d] (score: %.4f)\n%s\n\n", i+1, score, content)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&prompt, "prompt", "", "Query prompt (required)")
	cmd.Flags().IntVar(&topK, "top-k", 3, "Retrieval count")
	cmd.MarkFlagRequired("prompt")
	return cmd
}

func newRAGEvalCmd() *cobra.Command {
	var wait bool
	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Trigger Ragas evaluation",
		RunE: func(cmd *cobra.Command, args []string) error {
			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			ctx := context.Background()

			cronJob, err := kc.Clientset.BatchV1().CronJobs(kc.Namespace).Get(ctx, "kube-llmops-ragas-eval", metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("Ragas CronJob not found: %w", err)
			}

			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "ragas-manual-",
					Namespace:    kc.Namespace,
				},
				Spec: cronJob.Spec.JobTemplate.Spec,
			}
			created, err := kc.Clientset.BatchV1().Jobs(kc.Namespace).Create(ctx, job, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create eval job: %w", err)
			}
			fmt.Fprintf(os.Stdout, "Ragas evaluation job %q created\n", created.Name)

			if wait {
				fmt.Fprintln(os.Stdout, "Waiting for completion...")
				for {
					j, err := kc.Clientset.BatchV1().Jobs(kc.Namespace).Get(ctx, created.Name, metav1.GetOptions{})
					if err != nil {
						return err
					}
					if j.Status.Succeeded > 0 {
						fmt.Fprintln(os.Stdout, "Evaluation completed successfully.")
						return nil
					}
					if j.Status.Failed > 0 {
						return fmt.Errorf("evaluation job failed")
					}
					time.Sleep(5 * time.Second)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait for completion")
	return cmd
}
