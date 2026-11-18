package main

import (
	"YaxUnitMCP/service"
	DataBase "YaxUnitMCP/src/DB"
	mcpserver "YaxUnitMCP/src/MCP"
	YaxUnitParser "YaxUnitMCP/src/parser"
	"context"

	"io"
	"log"
	"os"
	"path/filepath"

	"time"
)

func main() {

	err := startLogger("logs.log")
	if err != nil {
		log.Fatal(err)
	}
	url := os.Getenv("ELASTICSEARCH_URL")
	port := os.Getenv("MAINPORT")

	if url == "" {
		log.Fatal("ELASTICSEARCH_URL is not defined!")
	}
	if port == "" {
		log.Fatal("the MAINPORT is not defined")
	}
	db, err := DataBase.NewElasticDataBase(url)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Data base init sucses!")

	go ParserWork(db)

	mcpServer, err := mcpserver.New(db)

	if err != nil {
		log.Fatal(err)
	}

	log.Println("Server started!")
	err = mcpServer.Run(context.Background(), port)
	if err != nil {
		log.Fatal(err)
	}
}

func ParserWork(db DataBase.SearchDataBase) {

	PageCounter := 0
	MethodsCounter := 0
	YaxUnitParser.ParseAll(func(page service.PageData) {

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := db.IndexPage(ctx, page)
		if err != nil {
			log.Println(err)
			return
		}
		PageCounter++
		MethodsCounter += len(page.Module.Methods)
	})
	log.Printf("Downloading %v pages is done! Was indexed %v methods!", PageCounter, MethodsCounter)
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
