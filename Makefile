ci: docker-ci

docker-ci:
	@echo "--- build all the things"
	@docker run --rm \
		-v $$(pwd):/src/$$(basename $$(pwd)) \
		-w /src/$$(basename $$(pwd)) -t golang make test
.PHONY: docker-ci

test: 
	@GO111MODULE=on go test -cover -v ./...
