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
	chunkRequestLimit    = 1600 / textChunkSize
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
	trimmedText := strings.TrimSpace(text)
	lines := strings.Split(trimmedText, "\n")
	allWordCount := len(strings.Split(trimmedText, " "))
	chunks := make([]string, 0, allWordCount+10/textChunkSize)

	currentChunk := make([]string, 0, textChunkSize)
	lastLine := ""
	for _, line := range lines {
		lineWords := strings.Split(line, " ")
		if len(currentChunk)+len(lineWords) > textChunkSize {
			chunks = append(chunks, lastLine+strings.Join(currentChunk, " "))
			currentChunk = make([]string, 0, textChunkSize)
		}
		currentChunk = append(currentChunk, lineWords...)
		currentChunk[len(currentChunk)-1] += "\n"
		lastLine = line + "\n"
	}
	if len(currentChunk) > 0 {
		chunks = append(chunks, lastLine+strings.Join(currentChunk, " "))
	}

	highestCurrentIndex := 0
	collectionExists := false
	_, err := c.wc.Schema().ClassGetter().WithClassName(collectionName).Do(context.Background())
	if err == nil {
		collectionExists = true
		highestCurrentIndex, err = c.HighestChunkIndex(collectionName)
		if err != nil {
			return err
		}
	} else if weaveErr, ok := err.(*fault.WeaviateClientError); !ok || weaveErr.StatusCode != 404 {
		// Unexpected or not 404 error means something went terribly wrong
		return weaveErr
	}

	if !collectionExists {
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
	}

	batchJob := c.wc.Batch().ObjectsBatcher()

	propertyObjs := make([]*models.Object, len(chunks))
	for i, chunk := range chunks {
		propertyObjs[i] = &models.Object{
			Class: collectionName,
			Properties: map[string]any{
				string(Chunk):      chunk,
				string(ChunkIndex): i + highestCurrentIndex,
			},
		}
	}

	_, err = batchJob.WithObjects(propertyObjs...).Do(context.Background())
	return err
}

// HighestChunkIndex for given collection.
func (c *Client) HighestChunkIndex(collectionName string) (int, error) {
	indexRes, err := c.wc.GraphQL().Get().
		WithClassName(collectionName).
		WithFields(graphql.Field{Name: string(ChunkIndex)}).
		WithSort(graphql.Sort{Path: []string{string(ChunkIndex)}, Order: graphql.Desc}).
		WithLimit(1).
		Do(context.Background())
	if err != nil {
		return -1, err
	}
	if len(indexRes.Errors) > 0 {
		for _, e := range indexRes.Errors {
			err = errors.Join(err, errors.New(e.Message))
		}
		return -1, err
	}

	get := indexRes.Data["Get"].(map[string]interface{})
	col := get[collectionName].([]interface{})

	return int(col[0].(map[string]any)[string(ChunkIndex)].(float64)), nil
}

// PromptText performs a RAG query against the given collection by applying the prompt to each value matching the concepts.
func (c *Client) PromptText(collectionName, prompt string, searchConcepts ...string) (string, error) {
	res, err := c.wc.GraphQL().Get().
		WithClassName(collectionName).
		WithFields(graphql.Field{Name: string(Chunk)}).
		WithNearText(c.wc.GraphQL().NearTextArgBuilder().WithConcepts(searchConcepts)).
		WithLimit(chunkRequestLimit).
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

// SearchTranscripts for specified collection agains the search concepts. Returns a limited amount of stored chunk data.
func (c *Client) SearchTranscripts(collectionName string, searchConcepts ...string) ([]string, error) {
	res, err := c.wc.GraphQL().Get().
		WithClassName(collectionName).
		WithFields(graphql.Field{Name: string(Chunk)}).
		WithNearText(c.wc.GraphQL().NearTextArgBuilder().WithConcepts(searchConcepts)).
		WithLimit(chunkRequestLimit).
		Do(context.Background())
	if err != nil {
		return nil, err
	}
	if len(res.Errors) > 0 {
		for _, e := range res.Errors {
			err = errors.Join(err, errors.New(e.Message))
		}
		return nil, err
	}

	get := res.Data["Get"].(map[string]interface{})
	col := get[collectionName].([]interface{})
	allLines := make([]string, len(col))
	for i, data := range col {
		mapData := data.(map[string]interface{})
		allLines[i] = mapData[string(Chunk)].(string)
	}
	return allLines, nil
}
