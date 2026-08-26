package model

//go:generate go run ./distill -source openapi.json -overrides openapi-overrides.json -model-output ../generated.go -client-output ../client_generated.go
//go:generate gofmt -w ../generated.go ../client_generated.go
