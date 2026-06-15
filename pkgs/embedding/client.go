package embedding

import "context"

// Client generates vector embeddings for text inputs.
type Client interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimension(ctx context.Context) (int, error)
}

// TestResult summarizes a connectivity / embedding smoke test.
type TestResult struct {
	Dimension int       `json:"dimension"`
	Vector    []float32 `json:"vector,omitempty"`
	Sample    string    `json:"sample,omitempty"`
}
