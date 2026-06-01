package engine

import (
	"regexp"
	"strings"
)

// Precompiled regexes for MoE/VLM detection
var (
	moeQwenRe = regexp.MustCompile(`qwen3-\d+b-a\d+b`)
	vlmGLMRe  = regexp.MustCompile(`glm-\d+\.\d+v`)
)

// ResolveEngine determines the inference engine from the model source name.
// Backward-compatible wrapper: features=nil, defaultEngine="vllm".
func ResolveEngine(source, explicit string) string {
	return ResolveEngineEx(source, explicit, nil, "vllm")
}

// ResolveEngineEx determines the inference engine using capability-based resolution.
//
// Priority:
//  1. Explicit engine (if not "" or "auto") — user override
//  2. Source-based: GGUF/GUFF → llamacpp, embedding/reranker → tei
//  3. Feature tag: domestic-gpu → chitu, moe → sglang, vlm → sglang
//  4. Source auto-detect: known MoE/VLM patterns → sglang
//  5. Fallback: defaultEngine (typically "vllm")
func ResolveEngineEx(source, explicit string, features []string, defaultEngine string) string {
	if explicit != "" && explicit != "auto" {
		return explicit
	}
	s := strings.ToLower(source)

	// 1. GGUF/GUFF → llamacpp
	if strings.Contains(s, "gguf") || strings.Contains(s, "guff") {
		return "llamacpp"
	}
	// 2. Embedding/Reranker → tei
	if isEmbeddingOrReranker(s) {
		return "tei"
	}
	// 3. Feature: domestic-gpu → chitu
	if hasFeature(features, "domestic-gpu") {
		return "chitu"
	}
	// 4. Feature: moe → sglang
	if hasFeature(features, "moe") {
		return "sglang"
	}
	// 5. Feature: vlm → sglang
	if hasFeature(features, "vlm") {
		return "sglang"
	}
	// 6. Auto-detect MoE from source → sglang
	if IsMoESource(s) {
		return "sglang"
	}
	// 7. Auto-detect VLM from source → sglang
	if IsVLMSource(s) {
		return "sglang"
	}
	// 8. Default engine
	if defaultEngine == "" {
		return "vllm"
	}
	return defaultEngine
}

// ResolveModelType determines whether a model is an "llm", "embedding", or "reranker".
func ResolveModelType(source string) string {
	s := strings.ToLower(source)
	if strings.Contains(s, "rerank") {
		return "reranker"
	}
	if isEmbedding(s) {
		return "embedding"
	}
	return "llm"
}

// IsMoESource detects MoE architecture from a lowercased source name.
// Known MoE families:
//   - DeepSeek V3/V4/R1 (not distill variants, which are dense)
//   - Qwen3 MoE: *-NNNb-aNNNb (e.g., qwen3-235b-a22b)
//   - Mixtral
//   - GLM 4.5+ (MoE; glm-4-32b is dense)
//   - Kimi K2+
func IsMoESource(s string) bool {
	// DeepSeek MoE (V3, V4, R1 but not distill variants)
	if (strings.Contains(s, "deepseek-v3") || strings.Contains(s, "deepseek-v4") ||
		strings.Contains(s, "deepseek-r1")) && !strings.Contains(s, "distill") {
		return true
	}
	// Qwen3 MoE pattern: qwen3-235b-a22b
	if moeQwenRe.MatchString(s) {
		return true
	}
	// Mixtral
	if strings.Contains(s, "mixtral") {
		return true
	}
	// GLM 4.5+ (dot versions are MoE; glm-4-32b with dash is dense)
	if strings.Contains(s, "glm-4.") || strings.Contains(s, "glm-5") {
		return true
	}
	// Kimi K2+
	if strings.Contains(s, "kimi-k2") {
		return true
	}
	return false
}

// IsVLMSource detects Vision-Language Model from a lowercased source name.
// Known VLM patterns: *-vl-*, *-vlm*, *-vision*, GLM *V suffix.
func IsVLMSource(s string) bool {
	if strings.Contains(s, "-vl-") || strings.HasSuffix(s, "-vl") ||
		strings.Contains(s, "-vlm") || strings.Contains(s, "-vision") {
		return true
	}
	if vlmGLMRe.MatchString(s) {
		return true
	}
	return false
}

func hasFeature(features []string, tag string) bool {
	for _, f := range features {
		if f == tag {
			return true
		}
	}
	return false
}

func isEmbeddingOrReranker(s string) bool {
	if strings.Contains(s, "rerank") {
		return true
	}
	return isEmbedding(s)
}

func isEmbedding(s string) bool {
	patterns := []string{
		"/bge-", "/e5-", "/gte-",
		"minilm",
		"/jina-embed", "jina-embeddings",
		"/nomic-embed", "nomic-embed",
		"/all-mpnet",
		"embedding",
	}
	for _, p := range patterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
