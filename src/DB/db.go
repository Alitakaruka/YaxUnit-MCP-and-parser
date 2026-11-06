package DataBase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
)

type SearchDataBase interface {
	IndexDocument(context.Context, Document) error
	SearchDocuments(context.Context, string) (SearchResult, error)
	CountDocuments(context.Context) (int64, error)
}

type SearchResult struct {
	Total int64           `json:"total"`
	Hits  json.RawMessage `json:"hits"`
}

type elasticDataBase struct {
	es *elasticsearch.Client
}

type Document struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Category  string    `json:"category"`
	Tags      []string  `json:"tags"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewElasticDataBase(url string) (*elasticDataBase, error) {
	db := elasticDataBase{}

	es, err := elasticsearch.New(elasticsearch.WithAddresses(url))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	res, err := es.Info(es.Info.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf(
			"elasticsearch connection error: status=%s, body=%s",
			res.Status(),
			string(body),
		)
	}

	exists, err := es.Indices.Exists(
		[]string{"documents"},
		es.Indices.Exists.WithContext(ctx),
	)
	if err != nil {
		return nil, err
	}

	if exists.StatusCode == 404 {
		err = CreateIndex(es)
		if err != nil {
			return nil, err
		}
	}

	db.es = es

	return &db, nil
}

func CreateIndex(es *elasticsearch.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	resp, err := es.Indices.Create("documents", es.Indices.Create.WithContext(ctx))

	if err != nil {
		return err
	}

	if resp.IsError() {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(
			"elasticsearch search error: status=%s, body=%s",
			resp.Status(),
			string(body),
		)
	}
	return nil
}

func (E *elasticDataBase) CountDocuments(ctx context.Context) (int64, error) {
	res, err := E.es.Count(
		E.es.Count.WithContext(ctx),
		E.es.Count.WithIndex("documents"),
	)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return 0, fmt.Errorf(
			"elasticsearch count error: status=%s, body=%s",
			res.Status(),
			string(body),
		)
	}

	var result struct {
		Count int64 `json:"count"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return 0, err
	}

	return result.Count, nil
}

func (E *elasticDataBase) IndexDocument(ctx context.Context, doc Document) error {

	if len(doc.Content) == 0 {
		return errors.New("document is empty")
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	indexRes, err := E.es.Index(
		"documents",
		bytes.NewReader(body),
		E.es.Index.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer indexRes.Body.Close()

	if indexRes.IsError() {
		body, _ := io.ReadAll(indexRes.Body)
		return fmt.Errorf(
			"elasticsearch index error: status=%s, body=%s",
			indexRes.Status(),
			string(body),
		)
	}

	return nil
}

func (E *elasticDataBase) SearchDocuments(ctx context.Context, searchText string) (SearchResult, error) {

	query, err := json.Marshal(map[string]interface{}{
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query": searchText,
				"fields": []string{
					"title^3",
					"content",
					"tags",
				},
			},
		},
	})

	if err != nil {
		return SearchResult{}, err
	}
	findRes, err := E.es.Search(
		E.es.Search.WithContext(ctx),
		E.es.Search.WithIndex("documents"),
		E.es.Search.WithBody(bytes.NewReader(query)),
	)
	if err != nil {
		return SearchResult{}, err
	}
	defer findRes.Body.Close()

	if findRes.IsError() {
		body, _ := io.ReadAll(findRes.Body)
		return SearchResult{}, fmt.Errorf(
			"elasticsearch search error: status=%s, body=%s",
			findRes.Status(),
			string(body),
		)
	}
	body, err := io.ReadAll(findRes.Body)

	if err != nil {
		return SearchResult{}, err
	}

	result := SearchResult{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return SearchResult{}, err
	}

	return result, err
}
