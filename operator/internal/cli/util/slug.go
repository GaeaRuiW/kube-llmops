package util

import "strings"

// SlugFromSource derives a K8s-safe name from a HuggingFace model source.
// Takes the part after the last "/", lowercases, truncates to 63 chars.
func SlugFromSource(source string) string {
	name := source
	if i := strings.LastIndex(source, "/"); i >= 0 {
		name = source[i+1:]
	}
	name = strings.ToLower(name)
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}
