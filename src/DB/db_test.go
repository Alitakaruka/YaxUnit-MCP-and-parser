package DataBase

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elastic/go-elasticsearch/v9"
)

// FIX-04/05 TEST BEGIN: Verify Elasticsearch contracts without a running cluster.
func newTestDataBase(t *testing.T, handler http.HandlerFunc) *elasticDataBase {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		writer.Header().Set("X-Elastic-Product", "Elasticsearch")
		writer.Header().Set("Content-Type", "application/json")
		handler(writer, req)
	}))
	t.Cleanup(server.Close)
	client, err := elasticsearch.New(elasticsearch.WithAddresses(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	return &elasticDataBase{es: client}
}

func TestIndexDocumentUsesStableID(t *testing.T) {
	requests := 0
	db := newTestDataBase(t, func(writer http.ResponseWriter, req *http.Request) {
		requests++
		if req.Method != http.MethodPut || req.URL.Path != "/documents/_doc/stable-id" {
			t.Errorf("unexpected indexing request: %s %s", req.Method, req.URL.Path)
		}
		var doc Document
		if err := json.NewDecoder(req.Body).Decode(&doc); err != nil {
			t.Error(err)
		}
		if doc.ID != "stable-id" {
			t.Errorf("payload ID = %q, want stable-id", doc.ID)
		}
		writer.Write([]byte(`{"result":"updated"}`))
	})
	doc := Document{ID: "stable-id", Content: "documentation"}
	for attempt := 0; attempt < 2; attempt++ {
		if err := db.IndexDocument(context.Background(), doc); err != nil {
			t.Fatal(err)
		}
	}
	if requests != 2 {
		t.Fatalf("received %d requests, want 2", requests)
	}
}

func TestIndexDocumentRejectsMissingID(t *testing.T) {
	db := &elasticDataBase{}
	err := db.IndexDocument(context.Background(), Document{Content: "documentation"})
	if err == nil || !strings.Contains(err.Error(), "ID") {
		t.Fatalf("expected missing-ID error, got %v", err)
	}
}

func TestSearchDocumentsDecodesTotal(t *testing.T) {
	for _, total := range []int64{0, 2, 10001} {
		t.Run(fmtTotal(total), func(t *testing.T) {
			db := newTestDataBase(t, func(writer http.ResponseWriter, req *http.Request) {
				if req.URL.Path != "/documents/_search" {
					t.Errorf("unexpected search path: %s", req.URL.Path)
				}
				var query struct {
					TrackTotalHits bool `json:"track_total_hits"`
				}
				if err := json.NewDecoder(req.Body).Decode(&query); err != nil {
					t.Error(err)
				}
				if !query.TrackTotalHits {
					t.Error("search must request an exact total")
				}
				json.NewEncoder(writer).Encode(map[string]interface{}{
					"hits": map[string]interface{}{
						"total": map[string]interface{}{"value": total, "relation": "eq"},
						"hits":  []interface{}{},
					},
				})
			})
			result, err := db.SearchDocuments(context.Background(), "documentation")
			if err != nil {
				t.Fatal(err)
			}
			if result.Total != total {
				t.Fatalf("total = %d, want %d", result.Total, total)
			}
			if !json.Valid(result.Hits) || !strings.Contains(string(result.Hits), `"hits"`) {
				t.Fatalf("original hits envelope was not preserved: %s", result.Hits)
			}
		})
	}
}

func fmtTotal(total int64) string {
	encoded, _ := json.Marshal(total)
	return string(encoded)
}
// FIX-04/05 TEST END.

// FIX-06 TEST BEGIN: Do not treat a failed index check as a successful connection.
func TestNewElasticDataBaseChecksIndexStatus(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantError  bool
		wantCreate int
	}{
		{"existing", http.StatusOK, false, 0},
		{"missing", http.StatusNotFound, false, 1},
		{"forbidden", http.StatusForbidden, true, 0},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			creates := 0
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
				writer.Header().Set("X-Elastic-Product", "Elasticsearch")
				writer.Header().Set("Content-Type", "application/json")
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/":
					writer.Write([]byte(`{"version":{"number":"9.5.2"}}`))
				case req.Method == http.MethodHead && req.URL.Path == "/documents":
					writer.WriteHeader(testCase.status)
				case req.Method == http.MethodPut && req.URL.Path == "/documents":
					creates++
					writer.Write([]byte(`{"acknowledged":true}`))
				default:
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
					writer.WriteHeader(http.StatusBadRequest)
				}
			}))
			t.Cleanup(server.Close)
			_, err := NewElasticDataBase(server.URL)
			if (err != nil) != testCase.wantError {
				t.Fatalf("error = %v, wantError = %v", err, testCase.wantError)
			}
			if creates != testCase.wantCreate {
				t.Fatalf("created index %d times, want %d", creates, testCase.wantCreate)
			}
		})
	}
}
// FIX-06 TEST END.

// MCP-01 TEST BEGIN: Check bounded searches, validation and document lookups.
func TestSearchDocumentation(t *testing.T) {
	db := newTestDataBase(t, func(writer http.ResponseWriter, req *http.Request) {
		var query struct {
			Size    int      `json:"size"`
			Source  []string `json:"_source"`
			Query   struct {
				Bool struct {
					Filter []map[string]map[string]string `json:"filter"`
				} `json:"bool"`
			} `json:"query"`
		}
		if err := json.NewDecoder(req.Body).Decode(&query); err != nil {
			t.Error(err)
		}
		if query.Size != 5 || len(query.Source) != 3 {
			t.Errorf("unexpected search bounds: %+v", query)
		}
		if len(query.Query.Bool.Filter) != 1 || query.Query.Bool.Filter[0]["prefix"]["source.keyword"] != "https://bia-technologies.github.io/yaxunit/api/" {
			t.Error("API section filter was not applied")
		}
		writer.Write([]byte(`{"hits":{"total":{"value":1},"hits":[{"_id":"stable-id","_score":2,"_source":{"title":"API","source":"https://example.test/api","content":"must not be returned"},"highlight":{"content":["API example"]}}]}}`))
	})
	result, err := db.SearchDocumentation(context.Background(), DocumentationSearch{Query: " API ", Section: "api"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Documents) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Documents[0].ID != "stable-id" || result.Documents[0].Snippet != "API example" {
		t.Fatalf("unexpected summary: %+v", result.Documents[0])
	}
}

func TestSearchDocumentationRejectsInvalidInput(t *testing.T) {
	db := &elasticDataBase{}
	inputs := []DocumentationSearch{
		{Query: " "}, {Query: strings.Repeat("x", 501)},
		{Query: "API", Limit: -1}, {Query: "API", Limit: 21},
		{Query: "API", Section: "unknown"},
	}
	for _, input := range inputs {
		if _, err := db.SearchDocumentation(context.Background(), input); err == nil {
			t.Errorf("expected validation error for %+v", input)
		}
	}
}

func TestGetDocument(t *testing.T) {
	db := newTestDataBase(t, func(writer http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/documents/_doc/stable-id" {
			t.Errorf("unexpected lookup: %s %s", req.Method, req.URL.Path)
		}
		writer.Write([]byte(`{"found":true,"_source":{"title":"API","content":"Full page"}}`))
	})
	doc, err := db.GetDocument(context.Background(), "stable-id")
	if err != nil || doc.Content != "Full page" {
		t.Fatalf("document = %+v, error = %v", doc, err)
	}
}

func TestGetDocumentRejectsInvalidID(t *testing.T) {
	db := &elasticDataBase{}
	for _, id := range []string{"", "../documents", "https://example.test", strings.Repeat("x", 129)} {
		if _, err := db.GetDocument(context.Background(), id); err == nil {
			t.Errorf("expected validation error for %q", id)
		}
	}
}

func TestGetDocumentNotFound(t *testing.T) {
	db := newTestDataBase(t, func(writer http.ResponseWriter, req *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
		writer.Write([]byte(`{"found":false}`))
	})
	if _, err := db.GetDocument(context.Background(), "missing-id"); err != ErrDocumentNotFound {
		t.Fatalf("error = %v, want ErrDocumentNotFound", err)
	}
}
// MCP-01 TEST END.