package engine

import "strings"

// ResolveEngine determines the inference engine from the model source name.
// Priority: explicit engine (if not "" or "auto") > heuristic > fallback "vllm"
func ResolveEngine(source, explicit string) string {
	if explicit != "" && explicit != "auto" {
		return explicit
	}
	s := strings.ToLower(source)
	// Typo-tolerant: match both "gguf" and "guff" (some HF repos misspell the format)
	if strings.Contains(s, "gguf") || strings.Contains(s, "guff") {
		return "llamacpp"
	}
	if isEmbeddingOrReranker(s) {
		return "tei"
	}
	return "vllm"
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
