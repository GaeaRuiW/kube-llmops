package engine

import "testing"

func TestResolveEngine(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		explicit string
		want     string
	}{
		{"explicit vllm", "anything", "vllm", "vllm"},
		{"explicit tei", "anything", "tei", "tei"},
		{"explicit llamacpp", "anything", "llamacpp", "llamacpp"},
		{"auto falls through", "Qwen/Qwen2.5-7B", "auto", "vllm"},
		{"empty falls through", "Qwen/Qwen2.5-7B", "", "vllm"},
		{"gguf in name", "TheBloke/Llama-2-7B-GGUF", "", "llamacpp"},
		{"gguf suffix", "model-gguf", "", "llamacpp"},
		{"GGUF uppercase", "TheBloke/Model-GGUF-Q4", "", "llamacpp"},
		{"GUFF typo uppercase", "nohurry/gemma-4-26B-A4B-it-heretic-GUFF", "", "llamacpp"},
		{"guff typo lowercase", "owner/model-guff-q4", "", "llamacpp"},
		{"rerank model", "BAAI/bge-reranker-base", "", "tei"},
		{"rerank in name", "cross-encoder/ms-marco-rerank", "", "tei"},
		{"bge embedding", "BAAI/bge-small-en-v1.5", "", "tei"},
		{"e5 embedding", "intfloat/e5-large-v2", "", "tei"},
		{"gte embedding", "thenlper/gte-large", "", "tei"},
		{"minilm", "sentence-transformers/all-MiniLM-L6-v2", "", "tei"},
		{"jina embed", "jinaai/jina-embeddings-v2", "", "tei"},
		{"nomic embed", "nomic-ai/nomic-embed-text", "", "tei"},
		{"all-mpnet", "sentence-transformers/all-mpnet-base-v2", "", "tei"},
		{"embedding keyword", "BAAI/bge-large-embedding-v1", "", "tei"},
		{"standard llm", "Qwen/Qwen2.5-7B-Instruct", "", "vllm"},
		{"meta llama", "meta-llama/Llama-3-8B", "", "vllm"},
		{"mistral", "mistralai/Mistral-7B-v0.1", "", "vllm"},
		{"awq model", "cyankiwi/gemma-4-26B-A4B-it-AWQ-4bit", "", "vllm"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveEngine(tt.source, tt.explicit)
			if got != tt.want {
				t.Errorf("ResolveEngine(%q, %q) = %q, want %q", tt.source, tt.explicit, got, tt.want)
			}
		})
	}
}

func TestResolveModelType(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"reranker", "BAAI/bge-reranker-base", "reranker"},
		{"embedding bge", "BAAI/bge-small-en-v1.5", "embedding"},
		{"embedding e5", "intfloat/e5-large-v2", "embedding"},
		{"llm", "Qwen/Qwen2.5-7B-Instruct", "llm"},
		{"gguf is llm", "TheBloke/Model-GGUF", "llm"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveModelType(tt.source)
			if got != tt.want {
				t.Errorf("ResolveModelType(%q) = %q, want %q", tt.source, got, tt.want)
			}
		})
	}
}
