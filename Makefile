CURRENT_VERSION := $(shell cat internal/project/project.go | sed -Ee 's/const Version = "(.*)"/\1/' | tail -1)

build:
	go build -o exec/deckard internal/cmd/deckard/main.go

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
	go test -parallel 6 -v -short -covermode=atomic -coverprofile coverage.out ./... 2>&1 | tee gotest.out
	go tool cover -func coverage.out | tee coverage.txt

# Run all tests including integration tests
test:
	go test -v -p 1 -parallel 6 -covermode=atomic -coverprofile coverage.out ./... 2>&1 | tee gotest.out
	go tool cover -func coverage.out | tee coverage.txt

# Run only integration tests
integration-test:
	go test -v -p 1 -parallel 6 -run Integration -covermode=atomic -coverprofile coverage.out ./... 2>&1 | tee gotest.out
	go tool cover -func coverage.out | tee coverage.txt

gen-proto:
	protoc --proto_path=proto/ --go-grpc_out=./ --go-grpc_opt=paths=source_relative --go_out=./ --go_opt=paths=source_relative proto/deckard_service.proto

gen-java:
	mkdir -p java/src/main/proto && cp -r proto java/src/main/proto
	mvn $(MAVEN_CLI_OPTS) -f ./java/pom.xml clean package

gen-csharp:
	protoc --proto_path=proto/ -I=proto/ --csharp_out=csharp/src --csharp_opt=file_extension=.g.cs,base_namespace=Takenet,serializable proto/deckard_service.proto

gen-deploy-java:
	mkdir -p java/src/main/proto && cp -r proto java/src/main/proto
	mvn $(MAVEN_CLI_OPTS) -f ./java/pom.xml clean deploy

gen-mocks:
	go generate ./...

fmt:
	go mod tidy
	go fmt ./...
	go vet ./...
