.PHONY: install test check vendor vendor-add vendor-update

install: check test
	go install .

test:
	go test -v -cover ./...

check:
	go fmt ./...
	go vet ./...
	go list ./... | xargs -n1 golint