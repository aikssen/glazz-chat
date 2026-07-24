package models

import "testing"

func TestModelSlugIsStableAndSafe(t *testing.T) {
	t.Parallel()
	tests := []struct {
		upstream string
		want     string
	}{
		{upstream: "deepseek-v4-flash", want: "deepseek-v4-flash"},
		{upstream: "vendor/model:latest", want: "model-vendor-model-latest-1df05a1617de"},
		{upstream: "x", want: "model-x-2d711642b726"},
	}
	for _, test := range tests {
		if got := modelSlug(test.upstream); got != test.want {
			t.Errorf("modelSlug(%q) = %q, want %q", test.upstream, got, test.want)
		}
	}
}

func TestModelDisplayNameRecognizesCatalogFamilies(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"deepseek-v4-pro": "DeepSeek V4 Pro",
		"qwen3.7-max":     "Qwen3.7 Max",
		"glm-5.2":         "GLM 5.2",
		"minimax-m3":      "MiniMax M3",
		"mimo-v2.5-pro":   "MiMo V2.5 Pro",
	}
	for upstream, want := range tests {
		if got := modelDisplayName(upstream); got != want {
			t.Errorf("modelDisplayName(%q) = %q, want %q", upstream, got, want)
		}
	}
}
