PROTO_ROOT := proto
MODULE     := $(shell go list -m)
PROTOC     ?= protoc
GOBIN      := $(shell go env GOPATH)/bin
SWAG       ?= swag
SWAG_FLAGS := --outputTypes yaml --v3.1

# protoc-gen-go / protoc-gen-go-grpc должны быть в PATH.
export PATH := $(GOBIN):$(PATH)

PROTO_FILES := $(shell find $(PROTO_ROOT) -name '*.proto' | sed 's|^$(PROTO_ROOT)/||' | sort)

.PHONY: proto proto-clean proto-check tools swagger swagger-identity swagger-content reindex-posts

# Генерирует Go-код в pkg/proto.
# protoc должен быть установлен отдельно; плагины ставятся make tools.
proto:
	@mkdir -p pkg/proto
	$(PROTOC) -I $(PROTO_ROOT) \
		--go_out=. --go_opt=module=$(MODULE) \
		--go-grpc_out=. --go-grpc_opt=module=$(MODULE) \
		$(PROTO_FILES)

# Проверка, что сгенерированный код не разъехался с proto.
proto-check: proto
	@git diff --exit-code -- pkg/proto

proto-clean:
	rm -rf pkg/proto/identity

tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Каждый сервис пишет свою спецификацию в свою папку.
swagger: swagger-identity swagger-content

swagger-identity:
	@mkdir -p docs/identity
	$(SWAG) init -g cmd/identity/main.go -o docs/identity $(SWAG_FLAGS) --tags auth,profile,authors

swagger-content:
	@mkdir -p docs/content
	$(SWAG) init -g cmd/server/main.go -o docs/content $(SWAG_FLAGS) --tags posts,donations

# Пересчитывает post.search_text у существующих постов санитайзером из pkg/sanitizer
reindex-posts:
	go run ./cmd/reindex-posts --config-file=configs/content.yaml
