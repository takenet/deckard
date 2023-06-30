CURRENT_VERSION := $(shell cat internal/project/project.go | sed -Ee 's/const Version = "(.*)"/\1/' | tail -1)

build:
	go build -o exec/deckard internal/cmd/deckard/main.go

lint:
	make gen-mocks
	@if ! command -v golangci-lint > /dev/null; then echo "Installing golangci-lint" && go install -a -v github.com/golang/golangci/golangci-lint@latest; fi;

	golangci-lint run ./...

build-image:
	@echo Building imagem. Version: $(CURRENT_VERSION)
	export SOURCE_DATE_EPOCH=$(shell date +%s);\
	GO111MODULE=on ko publish -B -t $(CURRENT_VERSION) github.com/takenet/deckard/internal/cmd/deckard
	GO111MODULE=on ko publish -B -t latest github.com/takenet/deckard/internal/cmd/deckard

build-local-image:
	@echo Building local imagem. Version: $(CURRENT_VERSION)
	export SOURCE_DATE_EPOCH=$(shell date +%s);\
	GO111MODULE=on ko publish -B -t $(CURRENT_VERSION) --local github.com/takenet/deckard/internal/cmd/deckard

run:
	go run internal/cmd/deckard/main.go

# -parallel 6 because unit tests may run in parallel if t.Parallel() is called
# -p 1 since integration tests can not run in parallel

# Run only unit tests
unit-test:
	make gen-cert
	make gen-mocks
	@if ! command -v gotestsum > /dev/null; then echo "Installing gotestsum" && go install -a -v gotest.tools/gotestsum@latest; fi;

	gotestsum --junitfile junit.xml -- -p 6 --parallel 6 -v -short -covermode=atomic -coverprofile coverage.out ./... 2>&1 | tee gotest.out
	go tool cover -func coverage.out | tee coverage.txt

# Run all tests including integration tests
test:
	make gen-cert
	make gen-mocks
	@if ! command -v gotestsum > /dev/null; then echo "Installing gotestsum" && go install -a -v gotest.tools/gotestsum@latest; fi;

	gotestsum --junitfile junit.xml -- -v -p 1 -parallel 1 -covermode=atomic -coverprofile coverage.out ./... 2>&1 | tee gotest.out
	go tool cover -func coverage.out | tee coverage.txt

# Run only integration tests
integration-test:
	make gen-cert
	make gen-mocks
	@if ! command -v gotestsum > /dev/null; then echo "Installing gotestsum" && go install -a -v gotest.tools/gotestsum@latest; fi;

	gotestsum --junitfile junit.xml -- -v -p 1 -parallel 1 -run Integration -covermode=atomic -coverprofile coverage.out  ./... 2>&1 | tee gotest.out
	go tool cover -func coverage.out | tee coverage.txt

gen-proto:
	protoc --proto_path=proto/ --go-grpc_out=./ --go-grpc_opt=paths=source_relative --go_out=./ --go_opt=paths=source_relative proto/deckard_service.proto

gen-java:
	mkdir -p java/src/main/proto && cp -r proto java/src/main
	mvn $(MAVEN_CLI_OPTS) -f ./java/pom.xml clean package

gen-csharp:
	mkdir -p csharp/proto && cp -r proto csharp
	cd csharp; dotnet build --configuration Release

gen-mocks:
	@if ! command -v mockgen > /dev/null; then echo "Installing mockgen" && go install -a -v github.com/golang/mock/mockgen@latest; fi;

	go generate ./...

gen-docs:
# https://github.com/pseudomuto/protoc-gen-doc/pull/520 or https://github.com/pseudomuto/protoc-gen-doc/pull/486
	@if ! command -v protoc-gen-doc > /dev/null; then echo "Installing protoc-gen-doc" && go install -a -v github.com/lucasoares/protoc-gen-doc/cmd/protoc-gen-doc@change-module-name; fi;

	protoc --doc_out=./docs/ --doc_opt=docs/markdown.tmpl,proto.md proto/*

# Generate certificates for tests
gen-cert:
	cd internal/service/cert; ./gen.sh 2>/dev/null || echo "ERROR: Certificate generation failed"

fmt:
	go mod tidy
	go fmt ./...
	go vet ./...
