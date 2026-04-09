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

	// Models
	if len(platform.Spec.Models) > 0 {
		models := make([]interface{}, len(platform.Spec.Models))
		for i, m := range platform.Spec.Models {
			model := map[string]interface{}{
				"name":   m.Name,
				"source": m.Source,
			}
			if m.Engine != "" {
				model["engine"] = m.Engine
			}
			if m.Replicas > 0 {
				model["replicas"] = m.Replicas
			}
			res := map[string]interface{}{}
			// Always set GPU (0 means CPU-only; omitting defaults to 1 in chart)
			res["gpu"] = int(m.Resources.GPU)
			if m.Resources.CPU != "" {
				res["cpu"] = m.Resources.CPU
			}
			if m.Resources.Memory != "" {
				res["memory"] = m.Resources.Memory
			}
			model["resources"] = res
			if len(m.EngineArgs) > 0 {
				model["engineArgs"] = m.EngineArgs
			}
			models[i] = model
		}
		global["models"] = models
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
		"enabled": gw.Enabled,
	}
	if gw.Routing != "" {
		litellm["routingStrategy"] = gw.Routing
	}
	if gw.MasterKey != "" {
		litellm["masterKey"] = gw.MasterKey
	}
	if gw.Image.Tag != "" {
		litellm["image"] = map[string]interface{}{"tag": gw.Image.Tag}
	}
	if gw.LangfuseEnabled {
		litellm["langfuseEnabled"] = true
		litellm["langfusePublicKey"] = gw.LangfusePublicKey
		litellm["langfuseSecretKey"] = gw.LangfuseSecretKey
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

	// Headlamp
	vals["headlamp"] = map[string]interface{}{
		"enabled": platform.Spec.Headlamp.Enabled,
	}

	return vals
}
