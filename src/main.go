package main

import (
	YaxUnitParser "YaxUnitMCP/src/parser"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

func main() {
	startLogger("")
	go ParserWork()
	time.Sleep(time.Hour)
}

func ParserWork() {
	for {
		YaxUnitParser.ParseAll(func(Data YaxUnitParser.PageData) {
			fmt.Printf("Data: %v\n", Data)
		})
		<-time.After(time.Hour * 24)
	}
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
