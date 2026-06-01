package engine

import "testing"

func TestResolveEngine(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		explicit string
		want     string
	}{
		// Explicit engine overrides
		{"explicit vllm", "anything", "vllm", "vllm"},
		{"explicit tei", "anything", "tei", "tei"},
		{"explicit llamacpp", "anything", "llamacpp", "llamacpp"},
		{"explicit sglang", "anything", "sglang", "sglang"},
		{"explicit chitu", "anything", "chitu", "chitu"},

		// Auto detection — backward compat (no features, default vllm)
		{"auto falls through", "Qwen/Qwen2.5-7B", "auto", "vllm"},
		{"empty falls through", "Qwen/Qwen2.5-7B", "", "vllm"},

		// GGUF → llamacpp
		{"gguf in name", "TheBloke/Llama-2-7B-GGUF", "", "llamacpp"},
		{"gguf suffix", "model-gguf", "", "llamacpp"},
		{"GGUF uppercase", "TheBloke/Model-GGUF-Q4", "", "llamacpp"},
		{"GUFF typo uppercase", "nohurry/gemma-4-26B-A4B-it-heretic-GUFF", "", "llamacpp"},
		{"guff typo lowercase", "owner/model-guff-q4", "", "llamacpp"},

		// Embedding/Reranker → tei
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

		// Standard LLM → vllm (default)
		{"standard llm", "Qwen/Qwen2.5-7B-Instruct", "", "vllm"},
		{"meta llama", "meta-llama/Llama-3-8B", "", "vllm"},
		{"mistral", "mistralai/Mistral-7B-v0.1", "", "vllm"},
		{"awq model", "cyankiwi/gemma-4-26B-A4B-it-AWQ-4bit", "", "vllm"},
		{"qwen3 dense", "Qwen/Qwen3-8B", "", "vllm"},
		{"glm dense", "ZhipuAI/GLM-4-32B-0414", "", "vllm"},
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

func TestResolveEngineEx(t *testing.T) {
	tests := []struct {
		name          string
		source        string
		explicit      string
		features      []string
		defaultEngine string
		want          string
	}{
		// Feature: domestic-gpu → chitu
		{"feature domestic-gpu", "Qwen/Qwen3-8B", "", []string{"domestic-gpu"}, "vllm", "chitu"},
		{"feature domestic-gpu over moe auto", "deepseek-ai/DeepSeek-V3", "", []string{"domestic-gpu"}, "vllm", "chitu"},

		// Feature: moe → sglang
		{"feature moe", "Qwen/Qwen3-8B", "", []string{"moe"}, "vllm", "sglang"},
		{"feature moe override default", "Qwen/Qwen3-8B", "", []string{"moe"}, "chitu", "sglang"},

		// Feature: vlm → sglang
		{"feature vlm", "my-custom/vision-model", "", []string{"vlm"}, "vllm", "sglang"},

		// Auto-detect MoE from source → sglang
		{"deepseek-v3 moe", "deepseek-ai/DeepSeek-V3", "", nil, "vllm", "sglang"},
		{"deepseek-v3.1 moe", "deepseek-ai/DeepSeek-V3.1", "", nil, "vllm", "sglang"},
		{"deepseek-v3.2 moe", "deepseek-ai/DeepSeek-V3.2", "", nil, "vllm", "sglang"},
		{"deepseek-v4 moe", "deepseek-ai/DeepSeek-V4-Flash-FP8", "", nil, "vllm", "sglang"},
		{"deepseek-r1 moe", "deepseek-ai/DeepSeek-R1", "", nil, "vllm", "sglang"},
		{"deepseek-r1-distill dense", "deepseek-ai/DeepSeek-R1-Distill-Qwen-14B", "", nil, "vllm", "vllm"},
		{"deepseek-r1-distill-llama dense", "deepseek-ai/DeepSeek-R1-Distill-Llama-70B", "", nil, "vllm", "vllm"},
		{"qwen3 moe", "Qwen/Qwen3-235B-A22B", "", nil, "vllm", "sglang"},
		{"qwen3 moe fp8", "Qwen/Qwen3-235B-A22B-FP8", "", nil, "vllm", "sglang"},
		{"qwen3 30b moe", "Qwen/Qwen3-30B-A3B", "", nil, "vllm", "sglang"},
		{"mixtral", "mistralai/Mixtral-8x7B-Instruct-v0.1", "", nil, "vllm", "sglang"},
		{"glm-4.5 moe", "zai-org/GLM-4.5", "", nil, "vllm", "sglang"},
		{"glm-5 moe", "zai-org/GLM-5", "", nil, "vllm", "sglang"},
		{"glm-5.1 moe", "zai-org/GLM-5.1", "", nil, "vllm", "sglang"},
		{"kimi-k2", "moonshotai/Kimi-K2-Instruct", "", nil, "vllm", "sglang"},

		// Auto-detect VLM from source → sglang
		{"qwen vl", "Qwen/Qwen2.5-VL-7B-Instruct", "", nil, "vllm", "sglang"},
		{"llama vision", "meta-llama/Llama-3.2-11B-Vision-Instruct", "", nil, "vllm", "sglang"},
		{"glm vlm", "zai-org/GLM-4.5V", "", nil, "vllm", "sglang"},
		{"glm-4.6v", "zai-org/GLM-4.6V", "", nil, "vllm", "sglang"},

		// Explicit engine overrides all
		{"explicit beats feature", "anything", "vllm", []string{"moe"}, "sglang", "vllm"},
		{"explicit beats auto moe", "deepseek-ai/DeepSeek-V3", "chitu", nil, "vllm", "chitu"},

		// GGUF beats features
		{"gguf beats feature", "TheBloke/Model-GGUF", "", []string{"moe"}, "sglang", "llamacpp"},

		// Default engine
		{"default sglang", "Qwen/Qwen2.5-7B-Instruct", "", nil, "sglang", "sglang"},
		{"default chitu", "Qwen/Qwen2.5-7B-Instruct", "", nil, "chitu", "chitu"},
		{"default empty falls to vllm", "Qwen/Qwen2.5-7B-Instruct", "", nil, "", "vllm"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveEngineEx(tt.source, tt.explicit, tt.features, tt.defaultEngine)
			if got != tt.want {
				t.Errorf("ResolveEngineEx(%q, %q, %v, %q) = %q, want %q",
					tt.source, tt.explicit, tt.features, tt.defaultEngine, got, tt.want)
			}
		})
	}
}

func TestIsMoESource(t *testing.T) {
	tests := []struct {
		source string
		want   bool
	}{
		{"deepseek-ai/deepseek-v3", true},
		{"deepseek-ai/deepseek-v3.1", true},
		{"deepseek-ai/deepseek-r1", true},
		{"deepseek-ai/deepseek-r1-distill-qwen-14b", false},
		{"qwen/qwen3-235b-a22b", true},
		{"qwen/qwen3-30b-a3b", true},
		{"qwen/qwen3-8b", false},
		{"mistralai/mixtral-8x7b", true},
		{"zai-org/glm-4.5", true},
		{"zai-org/glm-5", true},
		{"zhipuai/glm-4-32b-0414", false},
		{"moonshotai/kimi-k2-instruct", true},
		{"qwen/qwen2.5-7b-instruct", false},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			if got := IsMoESource(tt.source); got != tt.want {
				t.Errorf("IsMoESource(%q) = %v, want %v", tt.source, got, tt.want)
			}
		})
	}
}

func TestIsVLMSource(t *testing.T) {
	tests := []struct {
		source string
		want   bool
	}{
		{"qwen/qwen2.5-vl-7b-instruct", true},
		{"meta-llama/llama-3.2-11b-vision-instruct", true},
		{"zai-org/glm-4.5v", true},
		{"zai-org/glm-4.6v", true},
		{"qwen/qwen3-8b", false},
		{"mistralai/mistral-7b", false},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			if got := IsVLMSource(tt.source); got != tt.want {
				t.Errorf("IsVLMSource(%q) = %v, want %v", tt.source, got, tt.want)
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
