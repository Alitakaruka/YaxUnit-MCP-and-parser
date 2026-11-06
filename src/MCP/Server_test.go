package mcpserver

import (
	"context"
	"errors"
	"testing"
	"time"
	"unicode/utf8"

	database "YaxUnitMCP/src/DB"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCP-02 TEST BEGIN: Read-only handlers must bound requests and preserve legacy support.
type readOnlyTestDB struct {
	count       int64
	document    database.Document
	input       database.DocumentationSearch
	deadline    bool
	writes      int
	reads       int
	err         error
}

func (db *readOnlyTestDB) IndexDocument(context.Context, database.Document) error {
	db.writes++
	return errors.New("unexpected write")
}

func (db *readOnlyTestDB) SearchDocuments(context.Context, string) (database.SearchResult, error) {
	return database.SearchResult{}, db.err
}

func (db *readOnlyTestDB) CountDocuments(ctx context.Context) (int64, error) {
	_, db.deadline = ctx.Deadline()
	db.reads++
	return db.count, db.err
}

func (db *readOnlyTestDB) SearchDocumentation(ctx context.Context, input database.DocumentationSearch) (database.DocumentationResults, error) {
	_, db.deadline = ctx.Deadline()
	db.input = input
	db.reads++
	return database.DocumentationResults{Total: 0, Documents: []database.DocumentationSummary{}}, db.err
}

func (db *readOnlyTestDB) GetDocument(ctx context.Context, id string) (database.Document, error) {
	_, db.deadline = ctx.Deadline()
	db.reads++
	return db.document, db.err
}

func TestGetDatabaseStatus(t *testing.T) {
	for _, count := range []int64{0, 3} {
		db := &readOnlyTestDB{count: count}
		server := &Server{db: db}
		_, output, err := server.getDatabaseStatus(context.Background(), nil, StatusInput{})
		if err != nil || output.DocumentCount != count || output.HasDocuments != (count > 0) {
			t.Fatalf("status = %+v, error = %v", output, err)
		}
		if !db.deadline || db.writes != 0 {
			t.Fatal("status handler must use a deadline and perform no writes")
		}
	}
}

func TestSearchDocumentationPassesOptions(t *testing.T) {
	db := &readOnlyTestDB{}
	server := &Server{db: db}
	_, output, err := server.searchDocumentation(context.Background(), nil, DocumentationInput{
		Query: "mock", Limit: 3, Section: "api",
	})
	if err != nil || output.Documents == nil {
		t.Fatalf("result = %+v, error = %v", output, err)
	}
	if db.input.Query != "mock" || db.input.Limit != 3 || db.input.Section != "api" || !db.deadline || db.writes != 0 {
		t.Fatalf("unexpected database request: %+v", db)
	}
}

func TestGetDocumentTruncatesUnicode(t *testing.T) {
	db := &readOnlyTestDB{document: database.Document{Content: "\u041f\u0440\u0438\u0432\u0435\u0442"}}
	server := &Server{db: db}
	_, output, err := server.getDocument(context.Background(), nil, DocumentInput{ID: "stable-id", MaxCharacters: 3})
	if err != nil || !output.Truncated || output.Content != "\u041f\u0440\u0438" || !utf8.ValidString(output.Content) {
		t.Fatalf("document = %+v, error = %v", output, err)
	}
	if !db.deadline || db.writes != 0 {
		t.Fatal("document handler must use a deadline and perform no writes")
	}
}

func TestGetDocumentRejectsInvalidBounds(t *testing.T) {
	db := &readOnlyTestDB{}
	server := &Server{db: db}
	inputs := []DocumentInput{{ID: " "}, {ID: "stable-id", MaxCharacters: -1}, {ID: "stable-id", MaxCharacters: 20001}}
	for _, input := range inputs {
		if _, _, err := server.getDocument(context.Background(), nil, input); err == nil {
			t.Errorf("expected error for %+v", input)
		}
	}
	if db.reads != 0 || db.writes != 0 {
		t.Fatal("invalid input must be rejected before accessing the database")
	}
}

func TestReadOnlyHandlersPropagateErrors(t *testing.T) {
	wantError := errors.New("database unavailable")
	server := &Server{db: &readOnlyTestDB{err: wantError}}
	_, _, statusErr := server.getDatabaseStatus(context.Background(), nil, StatusInput{})
	_, _, searchErr := server.searchDocumentation(context.Background(), nil, DocumentationInput{Query: "mock"})
	_, _, documentErr := server.getDocument(context.Background(), nil, DocumentInput{ID: "stable-id"})
	for _, err := range []error{statusErr, searchErr, documentErr} {
		if !errors.Is(err, wantError) {
			t.Errorf("error = %v, want wrapped database error", err)
		}
	}
}

func TestLegacyDatabaseStillSupported(t *testing.T) {
	legacy := struct{ database.SearchDataBase }{&readOnlyTestDB{}}
	server, err := New(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := server.getDatabaseStatus(context.Background(), nil, StatusInput{}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := server.searchDocumentation(context.Background(), nil, DocumentationInput{Query: "mock"}); err == nil {
		t.Fatal("legacy database must report unsupported structured search")
	}
}

func TestReadOnlyToolsOverMCP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server, err := New(&readOnlyTestDB{count: 3})
	if err != nil {
		t.Fatal(err)
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
	tools, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	wanted := map[string]bool{"get_database_status": false, "search_documentation": false, "get_document": false}
	for _, tool := range tools.Tools {
		if _, ok := wanted[tool.Name]; ok {
			wanted[tool.Name] = true
			if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
				t.Errorf("tool %s is not marked read-only", tool.Name)
			}
		}
	}
	for name, found := range wanted {
		if !found {
			t.Errorf("tool %s was not registered", name)
		}
	}
	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_database_status", Arguments: map[string]interface{}{},
	})
	if err != nil || result == nil || result.IsError || result.StructuredContent == nil {
		t.Fatalf("MCP result = %+v, error = %v", result, err)
	}
	for name, arguments := range map[string]map[string]interface{}{
		"search_documentation": {"query": "mock"},
		"get_document":         {"id": "stable-id"},
		"get_documentation":    {"query": "mock"},
	} {
		result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
		if err != nil || result == nil || result.IsError || result.StructuredContent == nil {
			t.Fatalf("tool %s: result = %+v, error = %v", name, result, err)
		}
	}
}
// MCP-02 TEST END.