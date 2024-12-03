.PHONY: all server agent clean

# 构建信息
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

# 输出目录
BIN_DIR := bin
SERVER_BIN := $(BIN_DIR)/mesh-server
AGENT_BIN := $(BIN_DIR)/mesh-agent

all: server agent

# 构建服务端
server:
	@echo "Building server..."
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(SERVER_BIN) ./cmd/server

# 构建客户端
agent:
	@echo "Building agent..."
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(AGENT_BIN) ./cmd/agent

# 清理构建产物
clean:
	@echo "Cleaning..."
	@rm -rf $(BIN_DIR)

# 运行服务端
run-server: server
	@echo "Running server..."
	@$(SERVER_BIN) -config configs/server.yaml

# 运行客户端
run-agent: agent
	@echo "Running agent..."
	@$(AGENT_BIN) -config configs/agent.yaml

# 生成协议文件
proto:
	@echo "Generating protocol files..."
	@protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/proto/task/*.proto

# 格式化代码
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# 运行测试
test:
	@echo "Running tests..."
	@go test -v ./...
