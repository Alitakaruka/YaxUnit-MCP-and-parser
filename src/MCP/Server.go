package mcp

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func CreateMCP() {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "YaxBase",
			Version: "1.0.0",
		},
		nil,
	)

	// Регистрируем инструменты
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_product",
		Description: "Получить информацию о товаре по ID",
	}, func(
		ctx context.Context,
		req *mcp.CallToolRequest,
		input struct{},
	) (*mcp.CallToolResult, map[string]int, error) {
		return nil, map[string]int{
			"total": 125,
		}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_stock",
		Description: "Получить общее количество товаров на складе",
	}, func(
		ctx context.Context,
		req *mcp.CallToolRequest,
		input struct{},
	) (*mcp.CallToolResult, map[string]int, error) {
		return nil, map[string]int{
			"total": 125,
		}, nil
	})

	log.Println("MCP Server started")

	if err := server.Run(
		context.Background(),
		&mcp.StdioTransport{},
	); err != nil {
		log.Fatal(err)
	}

}
