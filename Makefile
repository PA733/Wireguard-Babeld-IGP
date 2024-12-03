.PHONY: proto build clean

# 使用 go env 获取操作系统
GO_OS := $(shell go env GOOS)
GO_ARCH := $(shell go env GOARCH)

# 根据操作系统选择命令
ifeq ($(GO_OS),windows)
    RM = del /Q
    RM_RF = rmdir /S /Q
    EXE_EXT = .exe
    PROTO_DIR = api/proto
    GO_OUT_DIR = api/proto
else
    RM = rm -f
    RM_RF = rm -rf
    EXE_EXT = 
    PROTO_DIR = api/proto
    GO_OUT_DIR = api/proto
endif

# 目标平台交叉编译配置
PLATFORMS ?= windows linux darwin
ARCHS ?= amd64 arm64

# 生成 proto 文件
proto:
	protoc --go_out=. \
		--go-grpc_out=. \
		$(PROTO_DIR)/task.proto

# 生成 wire 文件
wire:
	wire ./cmd/server/

# 本地构建
build: proto
	go build -o bin/mesh-client$(EXE_EXT) ./cmd/server/

# 跨平台交叉编译
cross-build: proto
	@for PLATFORM in $(PLATFORMS); do \
		for ARCH in $(ARCHS); do \
			echo "Building for $${PLATFORM}/$$ARCH"; \
			GOOS=$${PLATFORM} GOARCH=$${ARCH} go build \
				-o bin/mesh-client-$${PLATFORM}-$${ARCH}$(if $(filter windows,${PLATFORM}),.exe) \
				./cmd/server/; \
		done \
	done

# 清理构建产物
clean:
	$(RM_RF) bin
	find $(GO_OUT_DIR) -name "*.pb.go" -delete

# Windows 特殊清理规则
ifeq ($(GO_OS),windows)
clean:
	if exist bin rmdir /S /Q bin
	del /S /Q $(subst /,\,$(GO_OUT_DIR))\*.pb.go
endif
