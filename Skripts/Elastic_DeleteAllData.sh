sudo curl -X POST "http://localhost:9200/documents/_delete_by_query?pretty"   -H "Content-Type: application/json"   -d '{
    "query": {
      "match_all": {}
    }
  }'

sudo curl -X DELETE "http://localhost:9200/documents?pretty"
