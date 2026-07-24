//go:build provider_smoke

package provider_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/provider"
)

func TestConfiguredDevelopmentProviderStreams(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider.Kind != "openai-compatible" {
		t.Skip("LLM_PROVIDER_KIND is not openai-compatible")
	}
	gateway, err := provider.NewOpenAICompatible(
		cfg.Provider.BaseURL,
		cfg.Provider.APIKey,
		nil,
		provider.Options{RequestTimeout: 60 * time.Second, FirstChunkTimeout: 30 * time.Second},
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stream, err := gateway.Stream(ctx, provider.Request{
		Model: cfg.Provider.DefaultModel,
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: "Follow the user request concisely."},
			{Role: provider.RoleUser, Content: "Reply with exactly: ok"},
		},
		MaxOutputTokens: 256,
		Temperature:     0,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	var content strings.Builder
	for {
		chunk, nextErr := stream.Next(ctx)
		if chunk.Text != "" {
			content.WriteString(chunk.Text)
		}
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			t.Fatal(nextErr)
		}
		if content.Len() > 1024 {
			t.Fatal("provider smoke response exceeded the bounded test size")
		}
	}
	if strings.TrimSpace(content.String()) == "" {
		t.Fatal("provider returned an empty streamed response")
	}
}
