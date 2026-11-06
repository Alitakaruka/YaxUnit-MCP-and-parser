# YaxBase MCP

MCP stdio server backed by Elasticsearch and the YaxUnit documentation parser.

## Docker

Install Docker Desktop with Linux containers and Docker Compose v2. Run commands
from this directory. Go is installed inside the build image, not on the host.
The first build and documentation download require internet access.

Validate configuration, then build and start both services:

```sh
docker compose config --quiet
docker compose up -d --build
docker compose ps
docker compose logs -f app
```

Elasticsearch is available at http://localhost:9200. The application waits for
its health check before starting. Documents are stored in a persistent Docker
volume.
<!-- FIX-03 BEGIN: Original import behavior retained for reference.
Initial indexing runs in the background when the index is empty; early
searches may return incomplete results. A nonempty index is not automatically
updated or repaired after an interrupted import.
-->
Documentation is refreshed in the background on every application startup;
early searches may return incomplete results. Stable URL-based IDs prevent new
duplicates and allow an interrupted import to recover on the next startup.
Existing documents with old automatically generated IDs are not deleted or
migrated; historical duplicates may remain.
<!-- FIX-03 END. -->

This configuration is for local development: Elasticsearch authentication is
disabled, and its host port is bound only to loopback. Do not expose this setup
publicly. If Elasticsearch reports insufficient `vm.max_map_count`, configure
the Docker Linux host/WSL VM according to Elasticsearch's requirements.

## MCP Client

The application uses stdin/stdout, not HTTP. It has no browser URL or HTTP port.
For an MCP client, use this mode instead of a detached application container:

```sh
docker compose stop app
docker compose up -d --wait elasticsearch
docker compose build app
```

Configure the client to launch `docker` with these arguments, replacing the
Compose path with the absolute path on your machine:

```json
{
  "mcpServers": {
    "yaxbase": {
      "command": "docker",
      "args": [
        "compose",
        "-f",
        "/absolute/path/to/YaxUnit-MCP-and-parser/compose.yaml",
        "run",
        "--rm",
        "--no-deps",
        "-T",
        "app"
      ]
    }
  }
}
```

On Windows, use forward slashes in the JSON path, for example `E:/projects/...`.
Elasticsearch must be running before the client connects. Avoid starting
multiple application containers during the initial import. Logs go to stderr
and to `/app/logs.log` inside the application container.

<!-- MCP-02 BEGIN: New tools are additive and read-only. -->
## MCP Tools

Existing tools, including `get_documentation`, are unchanged. Three read-only
tools have been added:

| Tool | Arguments | Output |
| --- | --- | --- |
| `get_database_status` | `{}` | `document_count`, `has_documents` |
| `search_documentation` | `query`, optional `limit` and `section` | `total`, `documents` with IDs, titles, source links, scores and snippets |
| `get_document` | `id`, optional `max_characters` | Title, source, content, and `truncated` |

For example, call `search_documentation` with:

```json
{"query": "mock", "limit": 5, "section": "api"}
```

Then pass an ID from its `documents` array to `get_document`:

```json
{"id": "ID_FROM_SEARCH", "max_characters": 10000}
```

Search defaults to 5 results and allows 1 to 20. Queries must contain 1 to 500
characters. `section` accepts `all` (default), `docs`, or `api`. Results contain
snippets of at most 300 characters, not complete page contents. Section filters
use the standard dynamic `source.keyword` mapping; custom mappings or URLs
exceeding that field's `ignore_above` limit may require using `section: "all"`.
No index mapping is changed by these tools.

Document lookup reads only Elasticsearch; it does not fetch arbitrary URLs.
`max_characters` defaults to 10000 and allows 1 to 20000, counted as Unicode
characters. `truncated` indicates when content was shortened. A missing ID
returns a tool error. All new tools have timeouts and the MCP `readOnlyHint`.
`has_documents` is not a signal that the documentation import has finished.
Documentation text is untrusted source material, not instructions to execute.

Search and document lookup use an optional `DocumentationReader` interface;
databases implementing only the original `SearchDataBase` retain their existing
tools and database status. New code is marked `MCP-01` (database) and `MCP-02`
(server), each with `BEGIN`/`END` and explanations. No old code was removed.

Rebuild the application image and reconnect your MCP client:

```sh
docker compose build app
```

Run the MCP and database tests with Go installed:

```sh
go test ./src/DB ./src/MCP
```

These tests use fake HTTP responses and an in-memory MCP connection, without
running Elasticsearch or accessing the documentation website.
<!-- MCP-02 END. -->

## Stop

```sh
docker compose down
```

The data volume is retained. To delete all indexed documents and the volume:

```sh
docker compose down -v
```

## Fix Markers

Each changed code block has `FIX-NN BEGIN` and `FIX-NN END` comments explaining
the change. Replaced code is retained in comments, including unused imports.

- `FIX-01`: wait for indexing callbacks and remove the unnecessary 24-hour delay.
- `FIX-02`: keep MCP stdout free of logging; use stderr and the log file.
- `FIX-03`: refresh partial imports on restart, bound indexing time, count successes.
- `FIX-04`: hash source URLs and use stable Elasticsearch document IDs.
- `FIX-05`: decode `hits.total.value` and request an exact total.
- `FIX-06`: close Elasticsearch response bodies and check index-existence errors.
- `FIX-07`: normalize pagination URLs without double escaping.

Offline regression tests use fake HTTP responses, not the live documentation
site or a running Elasticsearch cluster. With Go installed, run:

```sh
go test ./src/parser ./src/DB
go test -race ./src/parser ./src/DB
go vet ./src/...
```

The race detector requires a supported platform and a C toolchain.