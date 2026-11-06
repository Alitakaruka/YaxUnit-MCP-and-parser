package DataBase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"
)

// MCP-01 BEGIN: Optional read-only features leave SearchDataBase unchanged.
type DocumentationReader interface {
	SearchDocumentation(context.Context, DocumentationSearch) (DocumentationResults, error)
	GetDocument(context.Context, string) (Document, error)
}

type DocumentationSearch struct {
	Query   string
	Limit   int
	Section string
}

type DocumentationSummary struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Source  string   `json:"source"`
	Tags    []string `json:"tags"`
	Snippet string   `json:"snippet"`
	Score   float64  `json:"score"`
}

type DocumentationResults struct {
	Total     int64                  `json:"total"`
	Documents []DocumentationSummary `json:"documents"`
}

var ErrDocumentNotFound = errors.New("document not found")
var documentIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func (E *elasticDataBase) SearchDocumentation(ctx context.Context, input DocumentationSearch) (DocumentationResults, error) {
	input.Query = strings.TrimSpace(input.Query)
	if input.Query == "" || utf8.RuneCountInString(input.Query) > 500 {
		return DocumentationResults{}, errors.New("query must contain 1 to 500 characters")
	}
	if input.Limit == 0 {
		input.Limit = 5
	}
	if input.Limit < 1 || input.Limit > 20 {
		return DocumentationResults{}, errors.New("limit must be between 1 and 20")
	}
	input.Section = strings.ToLower(strings.TrimSpace(input.Section))
	if input.Section == "" {
		input.Section = "all"
	}
	if input.Section != "all" && input.Section != "docs" && input.Section != "api" {
		return DocumentationResults{}, errors.New("section must be all, docs or api")
	}

	boolQuery := map[string]interface{}{
		"must": []interface{}{map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query": input.Query, "fields": []string{"title^3", "content", "tags"},
			},
		}},
	}
	if input.Section != "all" {
		boolQuery["filter"] = []interface{}{map[string]interface{}{
			"prefix": map[string]interface{}{
				"source.keyword": "https://bia-technologies.github.io/yaxunit/" + input.Section + "/",
			},
		}}
	}
	body, err := json.Marshal(map[string]interface{}{
		"size": input.Limit, "track_total_hits": true,
		"_source": []string{"title", "source", "tags"},
		"query":   map[string]interface{}{"bool": boolQuery},
		"highlight": map[string]interface{}{
			"pre_tags": []string{""}, "post_tags": []string{""},
			"fields": map[string]interface{}{
				"content": map[string]interface{}{
					"fragment_size": 300, "number_of_fragments": 1, "no_match_size": 300,
				},
			},
		},
	})
	if err != nil {
		return DocumentationResults{}, err
	}
	res, err := E.es.Search(
		E.es.Search.WithContext(ctx),
		E.es.Search.WithIndex("documents"),
		E.es.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return DocumentationResults{}, err
	}
	defer res.Body.Close()
	if res.IsError() {
		return DocumentationResults{}, fmt.Errorf("documentation search failed: status=%s", res.Status())
	}
	var response struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				ID        string              `json:"_id"`
				Score     float64             `json:"_score"`
				Source    Document            `json:"_source"`
				Highlight map[string][]string `json:"highlight"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return DocumentationResults{}, err
	}
	result := DocumentationResults{
		Total: response.Hits.Total.Value, Documents: make([]DocumentationSummary, 0, len(response.Hits.Hits)),
	}
	for _, hit := range response.Hits.Hits {
		snippet := ""
		if fragments := hit.Highlight["content"]; len(fragments) > 0 {
			text := []rune(fragments[0])
			if len(text) > 300 {
				text = text[:300]
			}
			snippet = string(text)
		}
		tags := hit.Source.Tags
		if tags == nil {
			tags = []string{}
		}
		result.Documents = append(result.Documents, DocumentationSummary{
			ID: hit.ID, Title: hit.Source.Title, Source: hit.Source.Source,
			Tags: tags, Snippet: snippet, Score: hit.Score,
		})
	}
	return result, nil
}

func (E *elasticDataBase) GetDocument(ctx context.Context, id string) (Document, error) {
	id = strings.TrimSpace(id)
	if !documentIDPattern.MatchString(id) {
		return Document{}, errors.New("invalid document ID; use an ID returned by search_documentation")
	}
	res, err := E.es.Get("documents", id, E.es.Get.WithContext(ctx))
	if err != nil {
		return Document{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return Document{}, ErrDocumentNotFound
	}
	if res.IsError() {
		return Document{}, fmt.Errorf("document lookup failed: status=%s", res.Status())
	}
	var response struct {
		Found  bool     `json:"found"`
		Source Document `json:"_source"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return Document{}, err
	}
	if !response.Found {
		return Document{}, ErrDocumentNotFound
	}
	return response.Source, nil
}
// MCP-01 END: These methods issue only search and document-read requests.