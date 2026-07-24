package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxSSEEventBytes = 1 << 20

type OpenAICompatible struct {
	baseURL string
	apiKey  string
	client  *http.Client
	options Options
}

func NewOpenAICompatible(
	baseURL, apiKey string,
	client *http.Client,
	options Options,
) (*OpenAICompatible, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || !parsed.IsAbs() || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("provider base URL must be absolute HTTP(S)")
	}
	if apiKey == "" {
		return nil, errors.New("provider API key is required")
	}
	if client == nil {
		client = &http.Client{}
	}
	defaults := DefaultOptions()
	if options.RequestTimeout <= 0 {
		options.RequestTimeout = defaults.RequestTimeout
	}
	if options.FirstChunkTimeout <= 0 {
		options.FirstChunkTimeout = defaults.FirstChunkTimeout
	}
	return &OpenAICompatible{
		baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey,
		client: client, options: options,
	}, nil
}

func (gateway *OpenAICompatible) Catalog(ctx context.Context) ([]Model, error) {
	request, err := http.NewRequestWithContext(
		ctx, http.MethodGet, gateway.baseURL+"/models", nil,
	)
	if err != nil {
		return nil, err
	}
	gateway.authorize(request)
	response, err := gateway.client.Do(request)
	if err != nil {
		return nil, Normalize(err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, responseError(response)
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 2<<20))
	if err := decoder.Decode(&payload); err != nil {
		return nil, &Error{Code: CodeMalformed, Cause: err}
	}
	models := make([]Model, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.ID != "" {
			models = append(models, Model{ID: item.ID, ChatCompletions: true})
		}
	}
	return models, nil
}

func (gateway *OpenAICompatible) Stream(ctx context.Context, request Request) (Stream, error) {
	if err := validateRequest(request); err != nil {
		return nil, err
	}
	messages := make([]map[string]string, 0, len(request.Messages))
	for _, message := range request.Messages {
		messages = append(messages, map[string]string{
			"role": string(message.Role), "content": message.Content,
		})
	}
	payload := map[string]any{
		"model": request.Model, "messages": messages, "stream": true,
		"stream_options": map[string]bool{"include_usage": true},
		"max_tokens":     request.MaxOutputTokens,
	}
	if request.Temperature >= 0 {
		payload["temperature"] = request.Temperature
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, gateway.options.RequestTimeout)
	httpRequest, err := http.NewRequestWithContext(
		requestCtx, http.MethodPost, gateway.baseURL+"/chat/completions", bytes.NewReader(body),
	)
	if err != nil {
		cancel()
		return nil, err
	}
	gateway.authorize(httpRequest)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "text/event-stream")
	response, err := gateway.client.Do(httpRequest)
	if err != nil {
		cancel()
		return nil, Normalize(err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		cancel()
		return nil, responseError(response)
	}
	contentType := response.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(contentType), "text/event-stream") {
		response.Body.Close()
		cancel()
		return nil, &Error{Code: CodeMalformed, Cause: errors.New("provider response is not an event stream")}
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), maxSSEEventBytes)
	return &openAIStream{
		body: response.Body, scanner: scanner, cancel: cancel,
		requestID: response.Header.Get("X-Request-ID"),
	}, nil
}

func (gateway *OpenAICompatible) authorize(request *http.Request) {
	request.Header.Set("Authorization", "Bearer "+gateway.apiKey)
}

type openAIStream struct {
	body      io.ReadCloser
	scanner   *bufio.Scanner
	cancel    context.CancelFunc
	requestID string
	closed    bool
}

func (stream *openAIStream) Next(ctx context.Context) (Chunk, error) {
	if stream.closed {
		return Chunk{}, io.EOF
	}
	for stream.scanner.Scan() {
		select {
		case <-ctx.Done():
			return Chunk{}, ctx.Err()
		default:
		}
		line := strings.TrimSpace(stream.scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			stream.closed = true
			return Chunk{RequestID: stream.requestID}, io.EOF
		}
		var event struct {
			ID      string `json:"id"`
			Model   string `json:"model"`
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				PromptDetails    *struct {
					CachedTokens int `json:"cached_tokens"`
				} `json:"prompt_tokens_details"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return Chunk{}, &Error{Code: CodeMalformed, Cause: err}
		}
		chunk := Chunk{RequestID: firstNonEmpty(event.ID, stream.requestID), ProviderModel: event.Model}
		if len(event.Choices) > 0 {
			chunk.Text = event.Choices[0].Delta.Content
			if event.Choices[0].FinishReason != nil {
				chunk.FinishReason = normalizeFinish(*event.Choices[0].FinishReason)
			}
		}
		if event.Usage != nil {
			usage := Usage{
				InputTokens: event.Usage.PromptTokens, OutputTokens: event.Usage.CompletionTokens,
			}
			if event.Usage.PromptDetails != nil {
				usage.CachedTokens = event.Usage.PromptDetails.CachedTokens
			}
			chunk.Usage = &usage
		}
		if chunk.Text != "" || chunk.Usage != nil || chunk.FinishReason != "" {
			return chunk, nil
		}
	}
	if err := stream.scanner.Err(); err != nil {
		return Chunk{}, Normalize(err)
	}
	stream.closed = true
	return Chunk{}, io.ErrUnexpectedEOF
}

func (stream *openAIStream) Close() error {
	if stream.closed {
		stream.cancel()
		return stream.body.Close()
	}
	stream.closed = true
	stream.cancel()
	return stream.body.Close()
}

func responseError(response *http.Response) error {
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	code := CodeUnavailable
	retryable := response.StatusCode >= 500
	switch response.StatusCode {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		code = CodeInvalidRequest
	case http.StatusUnauthorized, http.StatusForbidden:
		code = CodeUnauthorized
	case http.StatusTooManyRequests:
		code = CodeRateLimited
		retryable = true
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		code = CodeTimeout
		retryable = true
	}
	return &Error{
		Code: code, Retryable: retryable, StatusCode: response.StatusCode,
		Cause: fmt.Errorf("provider HTTP status %d", response.StatusCode),
	}
}

func normalizeFinish(value string) FinishReason {
	switch value {
	case "stop":
		return FinishStop
	case "length":
		return FinishLength
	case "content_filter", "safety":
		return FinishSafety
	default:
		return FinishError
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

var _ Gateway = (*OpenAICompatible)(nil)
