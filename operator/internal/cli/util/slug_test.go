package util

import "testing"

func TestSlugFromSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"org/model", "Qwen/Qwen2.5-7B-Instruct", "qwen2.5-7b-instruct"},
		{"org/long-model", "cyankiwi/gemma-4-26B-A4B-it-AWQ-4bit", "gemma-4-26b-a4b-it-awq-4bit"},
		{"embedding", "BAAI/bge-small-en-v1.5", "bge-small-en-v1.5"},
		{"gguf", "TheBloke/Llama-2-7B-GGUF", "llama-2-7b-gguf"},
		{"no slash", "my-local-model", "my-local-model"},
		{"already lowercase", "org/model-name", "model-name"},
		{"multiple slashes", "a/b/c", "c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SlugFromSource(tt.source)
			if got != tt.want {
				t.Errorf("SlugFromSource(%q) = %q, want %q", tt.source, got, tt.want)
			}
		})
	}
}

func TestSlugFromSource_Truncation(t *testing.T) {
	source := "org/" + "abcdefghij" + "abcdefghij" + "abcdefghij" + "abcdefghij" + "abcdefghij" + "abcdefghij" + "abcdefghij"
	got := SlugFromSource(source)
	if len(got) > 63 {
		t.Errorf("slug length %d exceeds 63: %q", len(got), got)
	}
}

func TestSlugFromSource_SpecialChars(t *testing.T) {
	tests := []struct{ source, want string }{
		{"org/Model_v1.0", "model_v1.0"},
		{"org/Model (beta)", "model (beta)"},
	}
	for _, tt := range tests {
		if got := SlugFromSource(tt.source); got != tt.want {
			t.Errorf("SlugFromSource(%q) = %q, want %q", tt.source, got, tt.want)
		}
	}
}
