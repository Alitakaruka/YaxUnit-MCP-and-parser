package main

import (
	DataBase "YaxUnitMCP/src/DB"
	mcpserver "YaxUnitMCP/src/MCP"
	YaxUnitParser "YaxUnitMCP/src/parser"
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
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

	if count == 0 {
		go ParserWork(db)
	}

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
			ID:        strconv.Itoa(counter),
			Title:     data.Header,
			Content:   data.Data,
			Category:  "Documentation",
			Source:    data.URL,
			CreatedAt: time.Now(),
			Tags:      []string{"YaxUnit"},
		}
		err := db.IndexDocument(context.Background(), doc)
		if err != nil {
			log.Println(err)
		}
		counter++
	})
	log.Printf("Downloading %v pages is done!", counter)
	<-time.After(time.Hour * 24)
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
	muliWriter := io.MultiWriter(os.Stdout, file)
	log.SetFlags(log.Ltime | log.Ldate | log.Llongfile)
	log.SetOutput(muliWriter)
	return nil
}
