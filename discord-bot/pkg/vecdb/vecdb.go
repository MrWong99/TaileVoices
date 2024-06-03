package vecdb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/fault"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
)

// ChunkProperty for storing and retrieving text chunks.
type ChunkProperty string

const (
	// Chunk of text.
	Chunk ChunkProperty = "chunk"
	// ChunkIndex within this collection.
	ChunkIndex ChunkProperty = "chunk_index"
)

const (
	clientStartupTimeout = 3 * time.Second
	textChunkSize        = 200
	textChunkOverlap     = 50
)

var defaultClient *Client

// SetDefaultClient to override the DefaultClient().
func SetDefaultClient(client *Client) {
	defaultClient = client
}

// DefaultClient to connect to weaviate. Will be nil if SetDefaultClient was not called yet.
func DefaultClient() *Client {
	return defaultClient
}

// Client to interact with a weaviate instance.
type Client struct {
	wc weaviate.Client
}

// NewClient to interact with a weaviate instance.
func NewClient(scheme, addr string) (*Client, error) {
	cfg := weaviate.Config{
		Scheme: scheme,
		Host:   addr,
	}

	wc, err := weaviate.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	return &Client{wc: *wc}, wc.WaitForWeavaite(clientStartupTimeout)
}

// StoreText in the vector db. The text will automatically be chunked.
func (c *Client) StoreText(collectionName, text string) error {
	words := strings.Split(strings.TrimSpace(text), " ")
	chunks := make([]string, 0, len(words)/textChunkSize)
	for i := 0; i < len(words); i = i + textChunkSize - textChunkOverlap {
		if i < len(words)-textChunkSize {
			chunks = append(chunks, strings.Join(words[i:i+textChunkSize], " "))
			continue
		}
		chunks = append(chunks, strings.Join(words[i:], " "))
	}

	_, err := c.wc.Schema().ClassGetter().WithClassName(collectionName).Do(context.Background())
	if err == nil {
		return errors.New("collection already exists")
	} else if weaveErr, ok := err.(*fault.WeaviateClientError); !ok || weaveErr.StatusCode != 404 {
		// Unexpected or not 404 error means something went terribly wrong
		return weaveErr
	}

	err = c.wc.Schema().ClassCreator().WithClass(&models.Class{
		Class: collectionName,
		Properties: []*models.Property{
			{
				Name:     string(Chunk),
				DataType: []string{"text"},
			},
			{
				Name:     string(ChunkIndex),
				DataType: []string{"int"},
			},
		},
		Vectorizer: "text2vec-openai",
		ModuleConfig: map[string]any{
			"generative-openai": map[string]any{
				"model": "gpt-4",
			},
		},
	}).Do(context.Background())
	if err != nil {
		return err
	}

	batchJob := c.wc.Batch().ObjectsBatcher()

	propertyObjs := make([]*models.Object, len(chunks))
	for i, chunk := range chunks {
		propertyObjs[i] = &models.Object{
			Class: collectionName,
			Properties: map[string]any{
				string(Chunk):      chunk,
				string(ChunkIndex): i,
			},
		}
	}

	_, err = batchJob.WithObjects(propertyObjs...).Do(context.Background())
	return err
}

// PromptText performs a RAG query against the given collection by applying the prompt to each value matching the concepts.
func (c *Client) PromptText(collectionName, prompt string, searchConcepts ...string) (string, error) {
	res, err := c.wc.GraphQL().Get().
		WithClassName(collectionName).
		WithFields(graphql.Field{Name: string(Chunk)}).
		WithNearText(c.wc.GraphQL().NearTextArgBuilder().WithConcepts(searchConcepts)).
		WithLimit(3000 / textChunkSize).
		WithGenerativeSearch(graphql.NewGenerativeSearch().GroupedResult(prompt)).
		Do(context.Background())
	if err != nil {
		return "", err
	}
	if len(res.Errors) > 0 {
		for _, e := range res.Errors {
			err = errors.Join(err, errors.New(e.Message))
		}
		return "", err
	}

	get := res.Data["Get"].(map[string]interface{})
	col := get[collectionName].([]interface{})

	_additional := col[0].(map[string]interface{})["_additional"]

	generate := _additional.(map[string]interface{})["generate"].(map[string]interface{})
	generateErr := generate["error"]
	if generateErr != nil {
		return "", fmt.Errorf("unexpected error in generation response: %v", generateErr)
	}
	groupedResult := generate["groupedResult"].(string)
	return groupedResult, nil
}
