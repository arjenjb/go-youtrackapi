# go-youtrackapi

A generated Go client for the [YouTrack REST API](https://www.jetbrains.com/help/youtrack/devportal/api-intro.html).

## Usage

```go
import "github.com/arjenjb/go-youtrackapi"

client := youtrackapi.NewClient("https://example.youtrack.cloud/api", []byte("permanent-token"))

projects, err := client.ListProjects(ctx, youtrackapi.ListProjectsRequest{
	Fields: "id,name,shortName",
})
```

For OAuth or another authentication transport, use `NewClientWithHTTPClient` and supply an
`http.Client` that adds credentials to requests.

## Regenerating

The repository includes the YouTrack 2026.2 OpenAPI document and semantic overrides used by the
generator. Run:

```sh
go generate ./internal/codegen
```

The generated models and client methods are committed so consumers do not need the generator.
