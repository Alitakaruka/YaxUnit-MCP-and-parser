package DataBase

import (
	"YaxUnitMCP/service"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
)

type SearchDataBase interface {
	IndexPage(context.Context, service.PageData) error
	// SearchDocuments(context.Context, string) (SearchResultDocument, error)
	SearchMethods(context.Context, string) (SearchResultMethod, error)
	// CountDocuments(context.Context) (int64, error)
}

type elasticDataBase struct {
	es *elasticsearch.Client
}

type SearchResultMethod struct {
	Total int64            `json:"total"`
	Hits  []service.Method `json:"hits"`
}

type elasticSearchResponseMethod struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`

		Hits []service.Method `json:"hits"`
	} `json:"hits"`
}

// type SearchResultDocument struct {
// 	Total int64            `json:"total"`
// 	Hits  []service.PageData `json:"hits"`
// }

// type elasticSearchResponseDocument struct {
// 	Hits struct {
// 		Total struct {
// 			Value int64 `json:"value"`
// 		} `json:"total"`

// 		Hits []service.Method `json:"hits"`
// 	} `json:"hits"`
// }

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
	db.es = es

	return &db, nil
}

// func (E *elasticDataBase) CountDocuments(ctx context.Context) (int64, error) {
// 	res, err := E.es.Count(
// 		E.es.Count.WithContext(ctx),
// 		E.es.Count.WithIndex("documents"),
// 	)
// 	if err != nil {
// 		return 0, err
// 	}
// 	defer res.Body.Close()

// 	if res.IsError() {
// 		body, _ := io.ReadAll(res.Body)
// 		return 0, fmt.Errorf(
// 			"elasticsearch count error: status=%s, body=%s",
// 			res.Status(),
// 			string(body),
// 		)
// 	}

// 	var result struct {
// 		Count int64 `json:"count"`
// 	}

// 	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
// 		return 0, err
// 	}

// 	return result.Count, nil
// }

func (E *elasticDataBase) IndexPage(ctx context.Context, page service.PageData) error {
	return E.indexMethods(ctx, page.Module)
}

// func (E *elasticDataBase) IndexDocument(ctx context.Context, doc service.PageData) error {

// // for _, module := range doc.
// if len(doc.Content) == 0 {
// 	return errors.New("document is empty")
// }
// if doc.ID == "" {
// 	return errors.New("document ID is empty")
// }
// body, err := json.Marshal(doc)
// if err != nil {
// 	return err
// }
// indexRes, err := E.es.Index(
// 	"documents",
// 	bytes.NewReader(body),
// 	E.es.Index.WithContext(ctx),
// 	E.es.Index.WithDocumentID(doc.ID),
// )
// if err != nil {
// 	return err
// }
// defer indexRes.Body.Close()

// if indexRes.IsError() {
// 	body, _ := io.ReadAll(indexRes.Body)
// 	return fmt.Errorf(
// 		"elasticsearch index error: status=%s, body=%s",
// 		indexRes.Status(),
// 		string(body),
// 	)
// }

// return nil
// }

func (E *elasticDataBase) indexMethods(ctx context.Context, module service.Module) error {
	// for _, module := range doc.
	if len(module.Methods) == 0 {
		return nil
	}

	for _, method := range module.Methods {
		method.Module = module.Name
		body, err := json.Marshal(method)
		if err != nil {
			return fmt.Errorf("marshal method: %w", err)
		}
		indexRes, err := E.es.Index(
			"methods",
			bytes.NewReader(body),
			E.es.Index.WithContext(ctx),
			E.es.Index.WithDocumentID(module.Name+"_"+method.Name),
		)
		if err != nil {
			return err
		}

		if indexRes.IsError() {
			body, _ := io.ReadAll(indexRes.Body)
			return fmt.Errorf(
				"elasticsearch index error: status=%s, body=%s",
				indexRes.Status(),
				string(body),
			)
		}
		indexRes.Body.Close()
	}
	return nil
}

func (E *elasticDataBase) SearchMethods(
	ctx context.Context,
	searchText string,
) (SearchResultMethod, error) {
	query, err := json.Marshal(map[string]interface{}{
		"track_total_hits": true,
		"size":             5,
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query": searchText,
				"fields": []string{
					"Name^5",
					"Description^2",
					"Module",
				},
			},
		},
	})
	if err != nil {
		return SearchResultMethod{}, fmt.Errorf("marshal query: %w", err)
	}

	res, err := E.es.Search(
		E.es.Search.WithContext(ctx),
		E.es.Search.WithIndex("documents"),
		E.es.Search.WithBody(bytes.NewReader(query)),
	)
	if err != nil {
		return SearchResultMethod{}, fmt.Errorf("elasticsearch search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return SearchResultMethod{}, fmt.Errorf(
			"elasticsearch search: status=%s, body=%s",
			res.Status(),
			body,
		)
	}

	var response elasticSearchResponseMethod

	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return SearchResultMethod{}, fmt.Errorf("decode search response: %w", err)
	}

	result := SearchResultMethod{
		Total: response.Hits.Total.Value,
		Hits:  make([]service.Method, 0, len(response.Hits.Hits)),
	}

	for _, hit := range response.Hits.Hits {
		result.Hits = append(result.Hits, hit)
	}

	return result, nil
}

// func (E *elasticDataBase) SearchDocuments(ctx context.Context, searchText string) (SearchResult, error) {

// 	query, err := json.Marshal(map[string]interface{}{
// 		"track_total_hits": true,
// 		"query": map[string]interface{}{
// 			"multi_match": map[string]interface{}{
// 				"query": searchText,
// 				"fields": []string{
// 					"title^3",
// 					"content",
// 					"tags",
// 				},
// 			},
// 		},
// 	})

// 	if err != nil {
// 		return SearchResult{}, err
// 	}
// 	findRes, err := E.es.Search(
// 		E.es.Search.WithContext(ctx),
// 		E.es.Search.WithIndex("documents"),
// 		E.es.Search.WithBody(bytes.NewReader(query)),
// 	)
// 	if err != nil {
// 		return SearchResult{}, err
// 	}
// 	defer findRes.Body.Close()

// 	if findRes.IsError() {
// 		body, _ := io.ReadAll(findRes.Body)
// 		return SearchResult{}, fmt.Errorf(
// 			"elasticsearch search error: status=%s, body=%s",
// 			findRes.Status(),
// 			string(body),
// 		)
// 	}
// 	body, err := io.ReadAll(findRes.Body)

// 	if err != nil {
// 		return SearchResult{}, err
// 	}
// 	var response struct {
// 		Hits json.RawMessage `json:"hits"`
// 	}
// 	if err := json.Unmarshal(body, &response); err != nil {
// 		return SearchResult{}, err
// 	}
// 	var hits struct {
// 		Total struct {
// 			Value int64 `json:"value"`
// 		} `json:"total"`
// 	}
// 	if err := json.Unmarshal(response.Hits, &hits); err != nil {
// 		return SearchResult{}, err
// 	}
// 	return SearchResult{Total: hits.Total.Value, Hits: response.Hits}, nil
// }
