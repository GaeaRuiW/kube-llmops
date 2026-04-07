package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kube-llmops/dashboard/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// finetuneResponse is the JSON shape the frontend expects.
type finetuneResponse struct {
	Metadata ftMeta   `json:"metadata"`
	Spec     ftSpec   `json:"spec"`
	Status   ftStatus `json:"status"`
}

type ftMeta struct {
	Name              string `json:"name"`
	Namespace         string `json:"namespace"`
	CreationTimestamp string `json:"creationTimestamp"`
}

type ftSpec struct {
	BaseModel  string `json:"baseModel"`
	OutputName string `json:"outputName"`
	Method     string `json:"method"`
}

type ftStatus struct {
	Phase          string `json:"phase"`
	ArgoWorkflow   string `json:"argoWorkflow"`
	StartTime      string `json:"startTime,omitempty"`
	CompletionTime string `json:"completionTime,omitempty"`
}

var workflowGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "workflows",
}

// ListFinetunes returns Argo Workflows that represent fine-tuning jobs.
func ListFinetunes(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}

		dynClient, err := dynamic.NewForConfig(kc.Config)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot create dynamic client: " + err.Error()})
			return
		}

		list, err := dynClient.Resource(workflowGVR).Namespace(kc.Namespace).List(c.Request.Context(), metav1.ListOptions{})
		if err != nil {
			// CRD might not be installed — return empty list gracefully
			c.JSON(http.StatusOK, []finetuneResponse{})
			return
		}

		var result []finetuneResponse
		for _, item := range list.Items {
			result = append(result, workflowToFinetune(item))
		}
		c.JSON(http.StatusOK, result)
	}
}

func workflowToFinetune(item unstructured.Unstructured) finetuneResponse {
	name := item.GetName()
	ns := item.GetNamespace()
	ts := item.GetCreationTimestamp().Format("2006-01-02T15:04:05Z")

	phase, _, _ := unstructured.NestedString(item.Object, "status", "phase")
	startedAt, _, _ := unstructured.NestedString(item.Object, "status", "startedAt")
	finishedAt, _, _ := unstructured.NestedString(item.Object, "status", "finishedAt")

	// Try to extract base model from workflow spec arguments
	baseModel := ""
	method := ""
	args, found, _ := unstructured.NestedSlice(item.Object, "spec", "arguments", "parameters")
	if found {
		for _, arg := range args {
			if m, ok := arg.(map[string]interface{}); ok {
				argName, _ := m["name"].(string)
				argValue, _ := m["value"].(string)
				if argName == "base-model" || argName == "base_model" || argName == "baseModel" {
					baseModel = argValue
				}
				if argName == "method" || argName == "finetuning_type" {
					method = argValue
				}
			}
		}
	}

	return finetuneResponse{
		Metadata: ftMeta{Name: name, Namespace: ns, CreationTimestamp: ts},
		Spec:     ftSpec{BaseModel: baseModel, OutputName: name, Method: method},
		Status: ftStatus{
			Phase:          phase,
			ArgoWorkflow:   name,
			StartTime:      startedAt,
			CompletionTime: finishedAt,
		},
	}
}

// CreateFinetune creates a new Argo Workflow for fine-tuning from JSON body.
func CreateFinetune(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		dynClient, err := dynamic.NewForConfig(kc.Config)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var body map[string]interface{}
		if err := json.NewDecoder(c.Request.Body).Decode(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		obj := &unstructured.Unstructured{Object: body}
		obj.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "argoproj.io",
			Version: "v1alpha1",
			Kind:    "Workflow",
		})
		created, err := dynClient.Resource(workflowGVR).Namespace(kc.Namespace).Create(c.Request.Context(), obj, metav1.CreateOptions{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, workflowToFinetune(*created))
	}
}

// GetFinetune returns a single Argo Workflow by name.
func GetFinetune(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		name := c.Param("name")
		dynClient, err := dynamic.NewForConfig(kc.Config)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		item, err := dynClient.Resource(workflowGVR).Namespace(kc.Namespace).Get(c.Request.Context(), name, metav1.GetOptions{})
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, workflowToFinetune(*item))
	}
}

// DeleteFinetune deletes an Argo Workflow by name.
func DeleteFinetune(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		name := c.Param("name")
		dynClient, err := dynamic.NewForConfig(kc.Config)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := dynClient.Resource(workflowGVR).Namespace(kc.Namespace).Delete(c.Request.Context(), name, metav1.DeleteOptions{}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	}
}

// StreamFinetuneLogs streams logs from Argo Workflow pods via SSE.
func StreamFinetuneLogs(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		name := c.Param("name")
		pods, err := kc.Clientset.CoreV1().Pods(kc.Namespace).List(c.Request.Context(), metav1.ListOptions{
			LabelSelector: "workflows.argoproj.io/workflow=" + name,
		})
		if err != nil || len(pods.Items) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "no pods found"})
			return
		}
		// Pick the most recent pod
		podName := pods.Items[len(pods.Items)-1].Name
		tailLines := int64(100)
		containerName := "main"
		req := kc.Clientset.CoreV1().Pods(kc.Namespace).GetLogs(podName, &corev1.PodLogOptions{
			Follow:    true,
			TailLines: &tailLines,
			Container: containerName,
		})
		stream, err := req.Stream(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer stream.Close()

		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Flush()

		buf := make([]byte, 4096)
		for {
			n, err := stream.Read(buf)
			if n > 0 {
				c.SSEvent("log", string(buf[:n]))
				c.Writer.Flush()
			}
			if err != nil {
				if err != io.EOF {
					c.SSEvent("error", err.Error())
					c.Writer.Flush()
				}
				return
			}
		}
	}
}
