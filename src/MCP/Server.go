package mcpserver

import (
	"context"
	"fmt"

	database "YaxUnitMCP/src/DB"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Server struct {
	server *mcp.Server
	db     database.SearchDataBase
}

type Input struct {
	Name string `json:"name" jsonschema:"the name of the person to greet"`
}

type Output struct {
	Greeting string `json:"greeting" jsonschema:"the greeting to tell to the user"`
}

func SayHi(ctx context.Context, req *mcp.CallToolRequest, input Input) (
	*mcp.CallToolResult,
	Output,
	error,
) {
	return nil, Output{Greeting: "Hi " + input.Name}, nil
}

func New(db database.SearchDataBase) (*Server, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}

	s := &Server{
		db: db,
	}

	s.server = mcp.NewServer(
		&mcp.Implementation{
			Name:    "YaxBase",
			Version: "1.0.0",
		},
		nil,
	)

	s.registerTools()

	return s, nil
}

func (s *Server) registerTools() {
	mcp.AddTool(
		s.server,
		&mcp.Tool{
			Name:        "get_documentation",
			Description: "Получить информацию из технической документации",
		},
		s.getDocumentation,
	)
	mcp.AddTool(
		s.server,
		&mcp.Tool{
			Name:        "Скажи привет!",
			Description: "просто отправляет привет",
		}, SayHi)
}

type SearchInput struct {
	Query string `json:"query" jsonschema:"Поисковый запрос"`
}

func (s *Server) getDocumentation(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input SearchInput,
) (*mcp.CallToolResult, Output, error) {
	if input.Query == "" {
		return nil, Output{}, fmt.Errorf("query is empty")
	}

	result, err := s.db.SearchDocuments(ctx, input.Query)
	if err != nil {
		return nil, Output{}, fmt.Errorf(
			"documentation search failed: %w", err,
		)
	}

	return nil, Output{Greeting: string(result.Hits)}, nil
}

func (s *Server) Run(ctx context.Context) error {
	return s.server.Run(
		ctx,
		&mcp.StdioTransport{},
	)
}
