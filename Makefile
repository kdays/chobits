.PHONY: fmt test tidy

fmt:
	go fmt ./...

test:
	go test ./...

tidy:
	go mod tidy
