package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

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
	// mcp.AddTool(
	// 	s.server,
	// 	&mcp.Tool{
	// 		Name:        "get_documentation",
	// 		Description: "Получить информацию из технической документации",
	// 	},
	// 	s.getDocumentation,
	// )
	closedWorld := false
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_method",
		Description: "Получить метод по имени и модулю.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld},
	}, s.getMethod)
}

type SearchInput struct {
	Query string `json:"query" jsonschema:"Поисковый запрос"`
}

func (s *Server) getMethod(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input SearchInput,
) (*mcp.CallToolResult, string, error) {
	if input.Query == "" {
		return nil, "", fmt.Errorf("query is empty")
	}

	result, err := s.db.SearchMethods(ctx, input.Query)
	if err != nil {
		return nil, "", fmt.Errorf(
			"documentation search failed: %w", err,
		)
	}

	js, err := json.Marshal(result)
	if err != nil {
		return nil, "", fmt.Errorf(
			"documentation search failed: %w", err,
		)
	}

	return nil, string(js), nil
}

type StatusInput struct{}

type StatusOutput struct {
	DocumentCount int64 `json:"document_count" jsonschema:"Number of documents currently visible in the index"`
	HasDocuments  bool  `json:"has_documents" jsonschema:"Whether at least one document is visible; not an import completion signal"`
}

type DocumentationInput struct {
	Query   string `json:"query" jsonschema:"Search text, 1 to 500 characters"`
	Limit   int    `json:"limit,omitempty" jsonschema:"Maximum results, 1 to 20; default 5"`
	Section string `json:"section,omitempty" jsonschema:"Section: all, docs or api; default all"`
}

type DocumentInput struct {
	ID            string `json:"id" jsonschema:"Elasticsearch document ID returned by search_documentation, not a URL"`
	MaxCharacters int    `json:"max_characters,omitempty" jsonschema:"Maximum content characters, 1 to 20000; default 10000"`
}

type DocumentOutput struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Source    string `json:"source"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

func (s *Server) Run(ctx context.Context, port string) error {
	handler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			return s.server
		},
		nil,
	)

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	mux.HandleFunc("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)

	go func() {
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err

	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancel()

		return httpServer.Shutdown(shutdownCtx)
	}
}
