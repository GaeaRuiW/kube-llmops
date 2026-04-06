package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/operator/internal/cli/util"
)

// difySession holds a logged-in Dify console session.
type difySession struct {
	endpoint  string
	csrfToken string
	client    *http.Client
}

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

// newDifySession creates a logged-in session to the Dify console API.
func newDifySession(ctx context.Context, kc *util.KubeClients) (*difySession, error) {
	endpoint, err := findDifyEndpoint(ctx, kc)
	if err != nil {
		return nil, err
	}

	email, password, err := findDifyCredentials(ctx, kc)
	if err != nil {
		return nil, err
	}

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: 30 * time.Second, Jar: jar}

	b64pw := base64.StdEncoding.EncodeToString([]byte(password))
	loginBody, _ := json.Marshal(map[string]string{"email": email, "password": b64pw})
	loginReq, _ := http.NewRequestWithContext(ctx, "POST", endpoint+"/console/api/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(loginReq)
	if err != nil {
		return nil, fmt.Errorf("cannot reach Dify at %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Dify login failed (%d): %s", resp.StatusCode, string(body))
	}

	var csrf string
	for _, c := range jar.Cookies(loginReq.URL) {
		if c.Name == "csrf_token" {
			csrf = c.Value
			break
		}
	}
	if csrf == "" {
		return nil, fmt.Errorf("Dify login succeeded but no CSRF token returned")
	}

	return &difySession{endpoint: endpoint, csrfToken: csrf, client: client}, nil
}

// do performs an authenticated request to the Dify console API.
func (s *difySession) do(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, s.endpoint+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-CSRF-Token", s.csrfToken)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	} else if body != nil && method != "GET" {
		req.Header.Set("Content-Type", "application/json")
	}
	return s.client.Do(req)
}

func findDifyEndpoint(ctx context.Context, kc *util.KubeClients) (string, error) {
	// Try NodePort from LLMPlatform CR
	var lp v1alpha1.LLMPlatform
	if err := kc.CRClient.Get(ctx, types.NamespacedName{Name: "kube-llmops", Namespace: kc.Namespace}, &lp); err == nil {
		if lp.Spec.NodePort.Enabled && lp.Spec.NodePort.Host != "" {
			return fmt.Sprintf("http://%s:30501", lp.Spec.NodePort.Host), nil
		}
	}
	// Try NodePort service
	svc, err := kc.Clientset.CoreV1().Services(kc.Namespace).Get(ctx, "kube-llmops-dify-api-np", metav1.GetOptions{})
	if err == nil {
		for _, p := range svc.Spec.Ports {
			if p.NodePort > 0 {
				node, nerr := kc.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
				if nerr == nil && len(node.Items) > 0 {
					for _, addr := range node.Items[0].Status.Addresses {
						if addr.Type == "InternalIP" {
							return fmt.Sprintf("http://%s:%d", addr.Address, p.NodePort), nil
						}
					}
				}
			}
		}
	}
	// Fall back to ClusterIP
	_, err = kc.Clientset.CoreV1().Services(kc.Namespace).Get(ctx, "kube-llmops-dify-api", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("Dify is not available. Enable RAG module: kubectl llmops platform update --enable rag")
	}
	return fmt.Sprintf("http://kube-llmops-dify-api.%s.svc.cluster.local:5001", kc.Namespace), nil
}

// findDifyCredentials reads admin email/password from the dify-setup ConfigMap.
func findDifyCredentials(ctx context.Context, kc *util.KubeClients) (string, string, error) {
	// Check env overrides first
	if e := os.Getenv("DIFY_EMAIL"); e != "" {
		if p := os.Getenv("DIFY_PASSWORD"); p != "" {
			return e, p, nil
		}
	}

	cm, err := kc.Clientset.CoreV1().ConfigMaps(kc.Namespace).Get(ctx, "kube-llmops-dify-setup", metav1.GetOptions{})
	if err != nil {
		return "", "", fmt.Errorf("Dify setup ConfigMap not found. Set DIFY_EMAIL and DIFY_PASSWORD environment variables")
	}
	script, ok := cm.Data["setup.sh"]
	if !ok {
		return "", "", fmt.Errorf("setup.sh not found in dify-setup ConfigMap")
	}

	emailRe := regexp.MustCompile(`ADMIN_EMAIL="([^"]+)"`)
	pwRe := regexp.MustCompile(`ADMIN_PASSWORD="([^"]+)"`)
	emailMatch := emailRe.FindStringSubmatch(script)
	pwMatch := pwRe.FindStringSubmatch(script)
	if len(emailMatch) < 2 || len(pwMatch) < 2 {
		return "", "", fmt.Errorf("cannot parse admin credentials from dify-setup ConfigMap")
	}
	return emailMatch[1], pwMatch[1], nil
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
			sess, err := newDifySession(ctx, kc)
			if err != nil {
				return err
			}
			resp, err := sess.do(ctx, "GET", "/console/api/datasets?page=1&limit=100", nil, "")
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
			if len(data) == 0 {
				fmt.Fprintln(os.Stdout, "No knowledge bases found.")
				return nil
			}
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
			sess, err := newDifySession(ctx, kc)
			if err != nil {
				return err
			}
			payload := map[string]string{"name": args[0]}
			if description != "" {
				payload["description"] = description
			}
			if embeddingModel != "" {
				payload["embedding_model"] = embeddingModel
			}
			jsonBody, _ := json.Marshal(payload)
			resp, err := sess.do(ctx, "POST", "/console/api/datasets", bytes.NewReader(jsonBody), "")
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
			sess, err := newDifySession(ctx, kc)
			if err != nil {
				return err
			}

			kbID, err := findKBIDByName(ctx, sess, kbName)
			if err != nil {
				return err
			}

			for _, filePath := range files {
				if err := uploadFile(ctx, sess, kbID, filePath, chunkSize, chunkOverlap); err != nil {
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

func findKBIDByName(ctx context.Context, sess *difySession, name string) (string, error) {
	resp, err := sess.do(ctx, "GET", "/console/api/datasets?page=1&limit=100", nil, "")
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

func uploadFile(ctx context.Context, sess *difySession, kbID, filePath string, chunkSize, chunkOverlap int) error {
	// Step 1: Upload file to Dify file storage
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", filepath.Base(filePath))
	io.Copy(part, f)
	writer.WriteField("source", "datasets")
	writer.Close()

	uploadResp, err := sess.do(ctx, "POST", "/console/api/files/upload", &buf, writer.FormDataContentType())
	if err != nil {
		return err
	}
	defer uploadResp.Body.Close()
	uploadBody, _ := io.ReadAll(uploadResp.Body)
	if uploadResp.StatusCode >= 400 {
		return fmt.Errorf("file upload failed (%d): %s", uploadResp.StatusCode, string(uploadBody))
	}
	var uploadResult map[string]interface{}
	json.Unmarshal(uploadBody, &uploadResult)
	fileID, _ := uploadResult["id"].(string)
	if fileID == "" {
		return fmt.Errorf("file upload returned no file ID: %s", string(uploadBody))
	}

	// Step 2: Create document in dataset referencing the uploaded file
	docPayload := map[string]interface{}{
		"data_source": map[string]interface{}{
			"info_list": map[string]interface{}{
				"data_source_type": "upload_file",
				"file_info_list":   map[string]interface{}{"file_ids": []string{fileID}},
			},
		},
		"indexing_technique": "high_quality",
		"process_rule": map[string]interface{}{
			"mode": "custom",
			"rules": map[string]interface{}{
				"pre_processing_rules": []map[string]interface{}{
					{"id": "remove_extra_spaces", "enabled": true},
					{"id": "remove_urls_emails", "enabled": false},
				},
				"segmentation": map[string]interface{}{
					"separator":     "\n",
					"max_tokens":    chunkSize,
					"chunk_overlap": chunkOverlap,
				},
			},
		},
	}
	docBody, _ := json.Marshal(docPayload)
	docResp, err := sess.do(ctx, "POST",
		fmt.Sprintf("/console/api/datasets/%s/documents", kbID),
		bytes.NewReader(docBody), "application/json")
	if err != nil {
		return err
	}
	defer docResp.Body.Close()
	if docResp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(docResp.Body)
		return fmt.Errorf("document creation failed (%d): %s", docResp.StatusCode, string(respBody))
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
			sess, err := newDifySession(ctx, kc)
			if err != nil {
				return err
			}

			kbID, err := findKBIDByName(ctx, sess, args[0])
			if err != nil {
				return err
			}

			delResp, err := sess.do(ctx, "DELETE", "/console/api/datasets/"+kbID, nil, "")
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
			sess, err := newDifySession(ctx, kc)
			if err != nil {
				return err
			}

			kbID, err := findKBIDByName(ctx, sess, args[0])
			if err != nil {
				return err
			}

			payload := map[string]interface{}{
				"query": prompt,
				"retrieval_model": map[string]interface{}{
					"search_method":           "semantic_search",
					"top_k":                   topK,
					"score_threshold_enabled": false,
					"reranking_enable":        false,
				},
			}
			jsonBody, _ := json.Marshal(payload)
			queryResp, err := sess.do(ctx, "POST", "/console/api/datasets/"+kbID+"/hit-testing", bytes.NewReader(jsonBody), "")
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
			if len(records) == 0 {
				fmt.Fprintln(os.Stdout, "No results found.")
				return nil
			}
			for i, r := range records {
				rec, _ := r.(map[string]interface{})
				segment, _ := rec["segment"].(map[string]interface{})
				content, _ := segment["content"].(string)
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
