package vecdb

import (
	"context"
	"fmt"
	"os"
	"testing"

	_ "embed"

	"github.com/weaviate/weaviate-go-client/v4/weaviate/fault"
)

func TestMain(m *testing.M) {
	if _, ok := os.LookupEnv("CI"); ok {
		// Disable test during CI for now
		os.Exit(0)
	}

	vecClient, err := NewClient("http", "localhost:8080")
	if err != nil {
		fmt.Printf("could not connect to weaviate instance: %v\n", err)
		os.Exit(1)
	}
	SetDefaultClient(vecClient)
	os.Exit(m.Run())
}

//go:embed transcript-example.txt
var exampleTranscript string

func TestStoreAndRetrieve(t *testing.T) {
	colName := "TestCollection"

	_, err := DefaultClient().wc.Schema().ClassGetter().WithClassName(colName).Do(context.Background())
	if err == nil {
		// Collection exists, cleaup needed
		DefaultClient().wc.Schema().ClassDeleter().WithClassName(colName).Do(context.Background())
	} else if weaveErr, ok := err.(*fault.WeaviateClientError); !ok || weaveErr.StatusCode != 404 {
		// Unexpected or not 404 error means something went terribly wrong
		t.Fatalf("unexpected error while operating weaviate: %v", err)
	}

	err = DefaultClient().StoreText(colName, exampleTranscript)
	if err != nil {
		t.Fatalf("could not store text data: %v", err)
	}

	index, err := DefaultClient().HighestChunkIndex(colName)
	if err != nil {
		t.Fatalf("could not get chunk index: %v", err)
	}

	if index != 97 {
		t.Errorf("expected highest index to be 97 and not %d", index)
	}

	result, err := DefaultClient().PromptText(colName, "Was hat Amon an diesem Tag alles erledigt? Antworte in ganz kurzen Stichpunkten!", "Amon")
	if err != nil {
		t.Fatalf("could not prompt text data: %v", err)
	}
	fmt.Printf("got resulting prompt:\n%s\n", result)
}
