build:
	go build -o bin/test-risk-agent ./cmd/test-risk-agent

test:
	go test ./...

fmt:
	gofmt -w cmd internal
