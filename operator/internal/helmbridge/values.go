package helmbridge

import (
	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
)

// TranslateValues converts an LLMPlatform CR spec into a Helm values map
// compatible with charts/kube-llmops-stack.
func TranslateValues(platform *v1alpha1.LLMPlatform) map[string]interface{} {
	vals := map[string]interface{}{}

	// Global section
	global := map[string]interface{}{}

	// Modules
	global["modules"] = map[string]interface{}{
		"rag":      map[string]interface{}{"enabled": platform.Spec.Modules.RAG.Enabled},
		"finetune": map[string]interface{}{"enabled": platform.Spec.Modules.Finetune.Enabled},
		"security": map[string]interface{}{"enabled": platform.Spec.Modules.Security.Enabled},
	}

	// ModelStore
	ms := platform.Spec.ModelStore
	global["modelStore"] = map[string]interface{}{
		"endpoint":              ms.Endpoint,
		"bucket":                ms.Bucket,
		"accessKey":             ms.AccessKey,
		"secretKey":             ms.SecretKey,
		"hfTransferConcurrency": ms.HFTransferConcurrency,
		"image":                 ms.Image,
	}

	// HF Token
	if platform.Spec.HFToken != "" {
		global["hfToken"] = platform.Spec.HFToken
	}

	// NodePort
	global["nodePort"] = map[string]interface{}{
		"enabled": platform.Spec.NodePort.Enabled,
		"host":    platform.Spec.NodePort.Host,
	}

	vals["global"] = global

	// LiteLLM (gateway)
	gw := platform.Spec.Gateway
	litellm := map[string]interface{}{
		"enabled":         gw.Enabled,
		"routingStrategy": gw.Routing,
	}
	if gw.Image.Tag != "" {
		litellm["image"] = map[string]interface{}{"tag": gw.Image.Tag}
	}
	vals["litellm"] = litellm

	// Observability
	vals["observability"] = map[string]interface{}{
		"enabled": platform.Spec.Observability.Enabled,
	}
	vals["langfuse"] = map[string]interface{}{
		"enabled": platform.Spec.Observability.Langfuse.Enabled,
	}

	// Logging
	vals["logging"] = map[string]interface{}{
		"enabled": platform.Spec.Logging.Enabled,
	}

	// Keycloak
	vals["keycloak"] = map[string]interface{}{
		"enabled": platform.Spec.Keycloak.Enabled,
	}

	// PostgreSQL
	vals["postgresql"] = map[string]interface{}{
		"enabled": platform.Spec.PostgreSQL.Enabled,
	}

	// KEDA
	vals["keda"] = map[string]interface{}{
		"enabled": platform.Spec.KEDA.Enabled,
	}

	// Fluid (MinIO)
	vals["fluid"] = map[string]interface{}{
		"enabled": ms.Enabled,
	}

	// Ingress
	if platform.Spec.Ingress.Enabled {
		vals["ingress"] = map[string]interface{}{
			"enabled":   true,
			"className": platform.Spec.Ingress.ClassName,
			"host":      platform.Spec.Ingress.Host,
		}
	}

	return vals
}
