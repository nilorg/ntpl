APP_NAME := ntpl
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GO_VERSION := $(shell go version | awk '{print $$3}')

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.buildTime=$(BUILD_TIME)

GOFLAGS := -trimpath
BIN_DIR := bin

.PHONY: build clean install test lint fmt vet tidy run-help

## build: 构建二进制文件
build:
	@mkdir -p $(BIN_DIR)
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(APP_NAME) .

## install: 安装到 $GOPATH/bin
install:
	go install $(GOFLAGS) -ldflags "$(LDFLAGS)" .

## clean: 清理构建产物
clean:
	rm -rf $(BIN_DIR)/
	rm -rf dist/

## test: 运行测试
test:
	go test ./... -v

## lint: 静态检查（需要 golangci-lint）
lint:
	golangci-lint run ./...

## fmt: 格式化代码
fmt:
	gofmt -s -w .

## vet: go vet 检查
vet:
	go vet ./...

## tidy: 整理依赖
tidy:
	go mod tidy

## cross: 交叉编译 linux/darwin/windows
cross:
	@mkdir -p dist
	GOOS=linux   GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-linux-arm64 .
	GOOS=darwin  GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-windows-amd64.exe .

## help: 显示帮助
help:
	@echo "可用目标："
	@grep -E '^## ' Makefile | sed 's/^## /  /' | column -t -s ':'
