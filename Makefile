.PHONY: build build-static build-compress build-static-compress run run-dev clean deps test release install-upx help

BINARY=cc-insights
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "")
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)"
BUILD_TAGS=-tags=prod
RELEASE_DIR=release
PACKAGE_DIR=package

# 默认目标
all: deps build

# 构建二进制文件（动态链接，开发用）
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
	@rm -rf $(BINARY) $(BINARY).original $(RELEASE_DIR) $(PACKAGE_DIR)
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

# ─── 发布版本（静态链接 + 全平台 UPX + 打包）─────────────────────────
# 产物格式: {name}_{version}_{os}_{arch}.{tar.gz|zip|exe}
# 所有平台均 CGO_ENABLED=0 静态链接，Windows 额外输出 UPX 压缩的 .exe
release: clean
	@echo "📦 构建发布版本（静态链接 + 全平台）..."
	@mkdir -p $(RELEASE_DIR) $(PACKAGE_DIR)
	@\
	for platform in \
		"linux:amd64:tar.gz" \
		"linux:arm64:tar.gz" \
		"darwin:amd64:tar.gz" \
		"darwin:arm64:tar.gz" \
		"windows:amd64:zip"; do \
		GOOS=$${platform%%:*}; \
		rest=$${platform#*:}; \
		GOARCH=$${rest%%:*}; \
		ARCHIVE=$${rest##*:}; \
		ext=""; \
		if [ "$$GOOS" = "windows" ]; then ext=".exe"; fi; \
		artifact_base="$(BINARY)_$(VERSION)_$${GOOS}_$${GOARCH}"; \
		package_dir="$(PACKAGE_DIR)/$${artifact_base}"; \
		mkdir -p "$$package_dir"; \
		echo "  → $$GOOS/$$GOARCH ($$ARCHIVE)..."; \
		CGO_ENABLED=0 go build -trimpath -tags=prod $(LDFLAGS) \
			-o "$$package_dir/$(BINARY)$$ext" ./cmd/insights; \
		cp README.md "$$package_dir/README.md" 2>/dev/null || true; \
		cp LICENSE "$$package_dir/LICENSE" 2>/dev/null || true; \
		if [ "$$ARCHIVE" = "zip" ]; then \
			(cd $(PACKAGE_DIR) && zip -r "../$(RELEASE_DIR)/$${artifact_base}.zip" "$${artifact_base}"); \
		else \
			tar -C $(PACKAGE_DIR) -czf "$(RELEASE_DIR)/$${artifact_base}.tar.gz" "$${artifact_base}"; \
		fi; \
		# Windows 额外输出 UPX 压缩的独立 exe \
		if [ "$$GOOS" = "windows" ] && which upx >/dev/null 2>&1; then \
			upx --best --lzma -o "$(RELEASE_DIR)/$${artifact_base}.exe" "$$package_dir/$(BINARY)$$ext" 2>/dev/null && \
			echo "    ↳ UPX 压缩: $$(ls -lh $(RELEASE_DIR)/$${artifact_base}.exe | awk '{print $$5}')"; \
		fi; \
		rm -rf "$$package_dir"; \
	done
	@echo ""
	@echo "📦 发布产物:"
	@ls -lh $(RELEASE_DIR)/
	@echo ""
	@cd $(RELEASE_DIR) && sha256sum * > checksums.txt
	@echo "📋 校验和已生成: $(RELEASE_DIR)/checksums.txt"

# 帮助
help:
	@echo "可用命令:"
	@echo "  make build                  - 构建二进制文件（动态链接，开发用）"
	@echo "  make build-static           - 构建静态链接版本（无外部依赖）"
	@echo "  make build-compress         - 构建并使用 UPX 压缩"
	@echo "  make build-static-compress  - 构建静态+UPX压缩（最小体积）"
	@echo "  make run                    - 构建并运行（使用默认数据目录）"
	@echo "  make run-dev                - 使用上级目录的数据运行（开发模式）"
	@echo "  make deps                   - 安装依赖"
	@echo "  make clean                  - 清理构建文件"
	@echo "  make test                   - 运行测试"
	@echo "  make bench                  - 性能测试（最近7天）"
	@echo "  make bench-all              - 性能测试（全部数据）"
	@echo "  make release                - 构建多平台发布版本（静态+UPX+打包+checksums）"
	@echo "  make install-upx            - 安装 UPX 压缩工具"
	@echo ""
	@echo "发布产物格式:"
	@echo "  {name}_{version}_{os}_{arch}.{tar.gz|zip}"
	@echo "  示例: cc-insights_v0.1.0_linux_amd64.tar.gz"
	@echo ""
	@echo "构建模式说明:"
	@echo "  动态链接: 体积小 (~6MB)，需要 glibc，适合标准 Linux 环境"
	@echo "  静态链接: 体积稍大 (~7MB)，无外部依赖，适合任意 Linux/容器"
	@echo "  UPX 压缩:  体积减少 ~68% (~2MB)，启动稍慢，适合分发"
	@echo ""
	@echo "参数说明:"
	@echo "  -data <path>  - 指定数据目录（默认: ./data）"
	@echo "  -addr <addr>  - 指定监听地址（默认: :8080）"
	@echo ""
	@echo "示例:"
	@echo "  make build-static          # 构建静态版本"
	@echo "  make build-static-compress # 构建静态压缩版本（推荐分发）"
	@echo "  make release               # 构建全平台发布包（推荐用于 GitHub Release）"
	@echo "  ./$(BINARY) -data /path/to/data -addr :9090"
