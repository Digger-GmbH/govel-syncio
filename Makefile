.PHONY: build run test tidy clean proto-gen proto-desc proto-all

BUILD_DIR=bin
SRC_DIR=./src
BINARY=govel-syncio

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY) $(SRC_DIR)

run: build
	./$(BUILD_DIR)/$(BINARY)

test:
	go test ./...

tidy:
	go mod tidy

clean:
	rm -rf $(BUILD_DIR)

proto-gen:
	protoc --proto_path=proto \
		--go_out=proto --go_opt=paths=source_relative \
		--go-grpc_out=proto --go-grpc_opt=paths=source_relative \
		proto/syncio.proto

proto-desc:
	protoc --proto_path=proto \
		--include_imports --include_source_info \
		--descriptor_set_out=proto/syncio.desc \
		proto/syncio.proto

proto-all: proto-gen proto-desc
