.PHONY: build build-static build-compress build-static-compress run run-dev clean deps test release release-static install-upx help

BINARY=cc-insights
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"
BUILD_TAGS=-tags=prod

# 默认目标
all: deps build

# 构建二进制文件
build:
	@echo "🔨 构建 $(BINARY)..."
	@go build -tags=prod $(LDFLAGS) -o $(BINARY) ./cmd/insights
	@echo "✅ 构建完成: ./$(BINARY)"

# 构建压缩版本（使用 UPX）
build-compress: build
	@echo "📦 使用 UPX 压缩..."
	@which upx > /dev/null || (echo "❌ UPX 未安装，请使用 'make install-upx' 安装" && exit 1)
	@cp $(BINARY) $(BINARY).original 2>/dev/null || true
	@upx --best --lzma $(BINARY)
	@echo "✅ 压缩完成"
	@echo "原始大小: $$(ls -lh $(BINARY).original | awk '{print $$5}')"
	@echo "压缩大小: $$(ls -lh $(BINARY) | awk '{print $$5}')"

# 构建静态链接版本（完全静态，无外部依赖）
build-static:
	@echo "🔨 构建 $(BINARY) (静态链接)..."
	@CGO_ENABLED=0 go build -tags=prod $(LDFLAGS) -o $(BINARY) ./cmd/insights
	@echo "✅ 静态构建完成: ./$(BINARY)"
	@echo "📦 大小: $$(ls -lh $(BINARY) | awk '{print $$5}')"
	@file $(BINARY) | grep -o "statically linked" && echo "✅ 确认: 完全静态链接" || echo "⚠️  警告: 可能不是完全静态"

# 构建静态链接压缩版本（静态 + UPX）
build-static-compress: build-static
	@echo "📦 使用 UPX 压缩静态版本..."
	@which upx > /dev/null || (echo "❌ UPX 未安装，请使用 'make install-upx' 安装" && exit 1)
	@cp $(BINARY) $(BINARY).original 2>/dev/null || true
	@upx --best --lzma $(BINARY)
	@echo "✅ 静态压缩完成"
	@echo "原始大小: $$(ls -lh $(BINARY).original | awk '{print $$5}')"
	@echo "压缩大小: $$(ls -lh $(BINARY) | awk '{print $$5}')"

# 运行（使用默认数据目录）
run: build
	@echo "🚀 启动 dashboard..."
	@./$(BINARY)

# 指定数据目录运行（开发模式）
run-dev:
	@echo "🚀 启动 dashboard (开发模式)..."
	@go run -tags=prod ./cmd/insights -data ../data

# 安装依赖
deps:
	@echo "📦 安装依赖..."
	@GOPROXY=https://goproxy.cn,direct go mod tidy
	@GOPROXY=https://goproxy.cn,direct go mod download

# 清理
clean:
	@echo "🧹 清理..."
	@rm -f $(BINARY) $(BINARY).original
	@go clean

# 测试
test:
	@echo "🧪 运行测试..."
	@go test $(BUILD_TAGS) -v ./...

# 性能测试
bench:
	@echo "🔍 性能测试 (最近7天)..."
	@go run -tags=bench ./cmd/insights -data ../data -range 7d

# 性能测试（全部数据）
bench-all:
	@echo "🔍 性能测试 (全部数据)..."
	@go run -tags=bench ./cmd/insights -data ../data -range all

# 安装 UPX（Ubuntu/Debian）
install-upx:
	@echo "📥 安装 UPX..."
	@sudo apt-get update && sudo apt-get install -y upx

# 发布版本（跨平台编译）
release: clean
	@echo "📦 构建发布版本（动态链接）..."
	@mkdir -p release
	@echo "  → Linux amd64..."
	@GOOS=linux GOARCH=amd64 go build -tags=prod $(LDFLAGS) -o release/$(BINARY)-linux-amd64 ./cmd/insights
	@echo "  → Linux arm64..."
	@GOOS=linux GOARCH=arm64 go build -tags=prod $(LDFLAGS) -o release/$(BINARY)-linux-arm64 ./cmd/insights
	@echo "  → macOS amd64..."
	@GOOS=darwin GOARCH=amd64 go build -tags=prod $(LDFLAGS) -o release/$(BINARY)-darwin-amd64 ./cmd/insights
	@echo "  → macOS arm64 (Apple Silicon)..."
	@GOOS=darwin GOARCH=arm64 go build -tags=prod $(LDFLAGS) -o release/$(BINARY)-darwin-arm64 ./cmd/insights
	@echo "  → Windows amd64..."
	@GOOS=windows GOARCH=amd64 go build -tags=prod $(LDFLAGS) -o release/$(BINARY)-windows-amd64.exe ./cmd/insights
	@ls -lh release/

# 发布版本（静态链接，完全便携）
release-static: clean
	@echo "📦 构建静态发布版本（完全静态，无外部依赖）..."
	@mkdir -p release
	@echo "  → Linux amd64 (static)..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags=prod $(LDFLAGS) -o release/$(BINARY)-linux-amd64-static ./cmd/insights
	@echo "  → Linux arm64 (static)..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags=prod $(LDFLAGS) -o release/$(BINARY)-linux-arm64-static ./cmd/insights
	@echo "  → Linux amd64 (static + UPX)..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags=prod $(LDFLAGS) -o release/$(BINARY)-linux-amd64-static.tmp ./cmd/insights && \
		upx --best --lzma -o release/$(BINARY)-linux-amd64-static.upx release/$(BINARY)-linux-amd64-static.tmp && \
		rm release/$(BINARY)-linux-amd64-static.tmp
	@echo "  → Linux arm64 (static + UPX)..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags=prod $(LDFLAGS) -o release/$(BINARY)-linux-arm64-static.tmp ./cmd/insights && \
		upx --best --lzma -o release/$(BINARY)-linux-arm64-static.upx release/$(BINARY)-linux-arm64-static.tmp && \
		rm release/$(BINARY)-linux-arm64-static.tmp
	@ls -lh release/

# 帮助
help:
	@echo "可用命令:"
	@echo "  make build              - 构建二进制文件（动态链接）"
	@echo "  make build-static       - 构建静态链接版本（无外部依赖）"
	@echo "  make build-compress     - 构建并使用 UPX 压缩（体积更小）"
	@echo "  make build-static-compress - 构建静态+UPX压缩（最小体积）"
	@echo "  make run                - 构建并运行（使用默认数据目录）"
	@echo "  make run-dev            - 使用上级目录的数据运行（开发模式）"
	@echo "  make deps               - 安装依赖"
	@echo "  make clean              - 清理构建文件"
	@echo "  make test               - 运行测试"
	@echo "  make bench              - 性能测试（最近7天）"
	@echo "  make bench-all          - 性能测试（全部数据）"
	@echo "  make release            - 构建多平台发布版本（动态链接）"
	@echo "  make release-static     - 构建多平台静态发布版本（完全便携）"
	@echo "  make install-upx        - 安装 UPX 压缩工具"
	@echo ""
	@echo "构建模式说明:"
	@echo "  动态链接: 体积小 (~6MB)，需要 glibc，适合标准 Linux 环境"
	@echo "  静态链接: 体积稍大 (~7MB)，无外部依赖，适合任意 Linux/容器"
	@echo "  UPX 压缩:  体积减少 68% (~2MB)，启动稍慢，适合分发"
	@echo ""
	@echo "参数说明:"
	@echo "  -data <path>  - 指定数据目录（默认: ./data）"
	@echo "  -addr <addr>  - 指定监听地址（默认: :8080）"
	@echo ""
	@echo "示例:"
	@echo "  make build-static          # 构建静态版本"
	@echo "  make build-static-compress # 构建静态压缩版本（推荐分发）"
	@echo "  ./$(BINARY) -data /path/to/data -addr :9090"
	@echo ""
	@echo "功能特性:"
	@echo "  ✅ 时间范围筛选 (7天/30天/90天/全部/自定义)"
	@echo "  ✅ 并发解析优化 (批量处理 + worker pool)"
	@echo "  ✅ 实时数据刷新 (AJAX 加载)"
	@echo "  ✅ 侧边栏交互界面"
	@echo "  ✅ 会话统计功能"
	@echo ""
	@echo "安装 UPX 压缩工具（可选）:"
	@echo "  sudo apt install upx       # Ubuntu/Debian"
	@echo "  brew install upx           # macOS"
