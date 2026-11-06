package main

import (
	DataBase "YaxUnitMCP/src/DB"
	mcpserver "YaxUnitMCP/src/MCP"
	YaxUnitParser "YaxUnitMCP/src/parser"
	"context"
	// FIX-04 BEGIN: Hash the source URL instead of using a download counter.
	"crypto/sha256"
	"fmt"
	// FIX-04 END.
	"io"
	"log"
	"os"
	"path/filepath"
	// FIX-04 BEGIN: Original counter-based ID import retained.
	// "strconv"
	// FIX-04 END.
	"time"
)

func main() {
	err := startLogger("logs.log")
	if err != nil {
		log.Fatal(err)
	}
	url := os.Getenv("ELASTICSEARCH_URL")
	if url == "" {
		url = "http://localhost:9200"
	}
	db, err := DataBase.NewElasticDataBase(url)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Data base init sucses!")
	count, err := db.CountDocuments(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// FIX-03 BEGIN: Retry the full import even when an earlier run was partial.
	/* Original startup condition:
	if count == 0 {
		go ParserWork(db)
	}
	*/
	log.Printf("Index contains %d documents; refreshing documentation", count)
	go ParserWork(db)
	// FIX-03 END: Stable document IDs make subsequent imports idempotent.

	// db.SearchDocuments(context.Background(), []byte("Как устроена система"))
	mcpServer, err := mcpserver.New(db)

	if err != nil {
		log.Fatal(err)
	}

	log.Println("Server started!")
	err = mcpServer.Run(context.Background())
	if err != nil {
		log.Fatal(err)
	}
}

func ParserWork(db DataBase.SearchDataBase) {

	counter := 0
	YaxUnitParser.ParseAll(func(data YaxUnitParser.PageData) {
		doc := DataBase.Document{
			// FIX-04 BEGIN: The same URL gets the same Elasticsearch document ID.
			// ID: strconv.Itoa(counter),
			ID: fmt.Sprintf("%x", sha256.Sum256([]byte(data.URL))),
			// FIX-04 END.
			Title:     data.Header,
			Content:   data.Data,
			Category:  "Documentation",
			Source:    data.URL,
			CreatedAt: time.Now(),
			Tags:      []string{"YaxUnit"},
		}
		// FIX-03 BEGIN: Bound indexing time so one failed request cannot stall import.
		// err := db.IndexDocument(context.Background(), doc)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := db.IndexDocument(ctx, doc)
		// FIX-03 END.
		if err != nil {
			log.Println(err)
			// FIX-03 BEGIN: Count only documents that were actually saved.
			return
			// FIX-03 END.
		}
		counter++
	})
	log.Printf("Downloading %v pages is done!", counter)
	// FIX-01 BEGIN: The parser now waits for callbacks; no artificial delay is needed.
	// <-time.After(time.Hour * 24)
	// FIX-01 END.
}

func startLogger(filePath string) error {
	dir := filepath.Dir(filePath)
	if dir != "." && dir != string(os.PathSeparator) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	// FIX-02 BEGIN: stdout belongs exclusively to the MCP stdio protocol.
	// Original logger before the Docker change:
	// muliWriter := io.MultiWriter(os.Stdout, file)
	muliWriter := io.MultiWriter(os.Stderr, file)
	// FIX-02 END: Diagnostics remain available on stderr and in the log file.
	log.SetFlags(log.Ltime | log.Ldate | log.Llongfile)
	log.SetOutput(muliWriter)
	return nil
}
