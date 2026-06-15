package llamacpp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"pkgs/db/models"
)

type Client struct {
	baseURL    string
	model      string
	httpClient *http.Client
	dimension  int
}

func NewClient(config models.EmbeddingConfig) *Client {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		baseURL = models.DefaultEmbeddingConfigBaseURL
	}
	model := strings.TrimSpace(config.ModelPath)
	if model == "" {
		model = "default"
	}
	return &Client{
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		dimension: config.Dimension,
	}
}

type openAIEmbeddingRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type openAIEmbeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	endpoint := ResolveEmbeddingEndpoint(c.baseURL)
	payload := openAIEmbeddingRequest{
		Input: texts,
		Model: c.model,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var parsed openAIEmbeddingResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("parse embedding response failed: %w", err)
	}
	if len(parsed.Data) == 0 {
		return nil, fmt.Errorf("embedding response is empty")
	}

	out := make([][]float32, len(texts))
	for _, item := range parsed.Data {
		if item.Index < 0 || item.Index >= len(texts) {
			continue
		}
		vec := make([]float32, len(item.Embedding))
		for i, value := range item.Embedding {
			vec[i] = float32(value)
		}
		out[item.Index] = vec
	}
	for i, vec := range out {
		if vec == nil {
			return nil, fmt.Errorf("missing embedding for input index %d", i)
		}
	}
	if c.dimension == 0 && len(out[0]) > 0 {
		c.dimension = len(out[0])
	}
	return out, nil
}

func (c *Client) Dimension(ctx context.Context) (int, error) {
	if c.dimension > 0 {
		return c.dimension, nil
	}
	vectors, err := c.Embed(ctx, []string{"dimension probe"})
	if err != nil {
		return 0, err
	}
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		return 0, fmt.Errorf("unable to detect embedding dimension")
	}
	c.dimension = len(vectors[0])
	return c.dimension, nil
}

func ResolveEmbeddingEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = models.DefaultEmbeddingConfigBaseURL
	}
	if strings.HasSuffix(baseURL, "/embeddings") {
		return baseURL
	}
	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL + "/embeddings"
	}
	if strings.HasSuffix(baseURL, "/embedding") {
		return baseURL
	}
	return baseURL + "/v1/embeddings"
}

func HealthCheck(ctx context.Context, config models.EmbeddingConfig) error {
	client := NewClient(config)
	_, err := client.Embed(ctx, []string{"health check"})
	return err
}

func Test(ctx context.Context, config models.EmbeddingConfig, sample string) (int, []float32, error) {
	sample = strings.TrimSpace(sample)
	if sample == "" {
		sample = "matrixops embedding test"
	}
	client := NewClient(config)
	vectors, err := client.Embed(ctx, []string{sample})
	if err != nil {
		return 0, nil, err
	}
	if len(vectors) == 0 {
		return 0, nil, fmt.Errorf("empty embedding result")
	}
	return len(vectors[0]), vectors[0], nil
}
