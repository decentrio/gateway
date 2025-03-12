all: install

LD_FLAGS = -w -s


BUILD_FLAGS := -ldflags '$(LD_FLAGS)'

build:
	@echo "Building gateway"
	@go build -mod readonly $(BUILD_FLAGS) -o build/gateway main.go

test:
	@echo "Testing basic"
	@go test -mod readonly --timeout=10m $(BUILD_FLAGS) `go list ./... |grep -v github.com/decentrio/gateway/test`

#test-osmosis:
#	@echo "Testing gateway with default osmosis config"
#	@go test -mod readonly --timeout=10m -ldflags '$(LD_FLAGS) -X github.com/decentrio/gateway/test.Chain=osmosis' ./test

test-evmos:
	@echo "Testing gateway with evmos config"
	@go test -p 1 -mod readonly --timeout=10m -ldflags '$(LD_FLAGS) -X github.com/decentrio/gateway/test.Chain=evmos' ./test

lint:
	@echo "Running golangci-lint"
	golangci-lint run --timeout=10m

install:
	@echo "Installing gateway"
	@go install -mod readonly $(BUILD_FLAGS) ./...

clean:
	rm -rf build

.PHONY: all lint test race msan tools clean build
