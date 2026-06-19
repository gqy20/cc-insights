.PHONY: web-build build run run-dev clean test bench bench-all release help

BINARY=cc-insights
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || "")
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)"
RELEASE_DIR=release
PACKAGE_DIR=package

# 默认目标
all: build

# 构建前端（React + Vite 产物落到 cmd/insights/static/dist，供 go:embed）
web-build:
	@echo "🎨 构建前端 (web/)..."
	@cd web && pnpm install --silent && pnpm run build
	@echo "✅ 前端构建完成 -> cmd/insights/static/dist/"

# 构建单二进制文件（静态链接 + UPX 压缩，可直接运行）
build: web-build
	@echo "🔨 构建 $(BINARY)..."
	@CGO_ENABLED=0 go build -trimpath -tags=prod $(LDFLAGS) -o $(BINARY) ./cmd/insights
	@echo "✅ 构建完成: ./$(BINARY) ($$(ls -lh $(BINARY) | awk '{print $$5}'))"
	@if which upx >/dev/null 2>&1; then \
		cp $(BINARY) $(BINARY).original 2>/dev/null || true; \
		upx --best --lzma $(BINARY); \
		echo "📦 UPX 压缩后: $$(ls -lh $(BINARY) | awk '{print $$5}')"; \
	fi

# 构建并运行（使用默认数据目录）
run: build
	@./$(BINARY)

# 开发模式运行（使用上级目录的数据，不生成二进制）
run-dev:
	@go run -tags=prod ./cmd/insights -data ../data

# 清理
clean:
	@rm -rf $(BINARY) $(BINARY).original $(RELEASE_DIR) $(PACKAGE_DIR)

# 测试
test:
	@go test -tags=prod -v ./...

# 性能测试（最近7天）
bench:
	@go run -tags=bench ./cmd/insights -data ../data -range 7d

# 性能测试（全部数据）
bench-all:
	@go run -tags=bench ./cmd/insights -data ../data -range all

# ─── 发布版本 ──────────────────────────────────────────────
# 产物: {name}_{version}_{os}_{arch}.{tar.gz|zip} + checksums.txt
release: clean
	@echo "📦 构建发布版本..."
	@mkdir -p $(RELEASE_DIR) $(PACKAGE_DIR)
	@\
	for platform in \
		"linux:amd64:tar.gz" "linux:arm64:tar.gz" \
		"darwin:amd64:tar.gz" "darwin:arm64:tar.gz" \
		"windows:amd64:zip"; do \
		GOOS=$${platform%%:*}; rest=$${platform#*:}; \
		GOARCH=$${rest%%:*}; ARCHIVE=$${rest##:*}; \
		ext=""; [ "$$GOOS" = "windows" ] && ext=".exe"; \
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
		if [ "$$GOOS" = "windows" ] && which upx >/dev/null 2>&1; then \
			upx --best --lzma -o "$(RELEASE_DIR)/$${artifact_base}.exe" "$$package_dir/$(BINARY)$$ext" 2>/dev/null && \
			echo "    ↳ UPX: $$(ls -lh $(RELEASE_DIR)/$${artifact_base}.exe | awk '{print $$5}')"; \
		fi; \
		rm -rf "$$package_dir"; \
	done
	@echo ""
	@echo "📦 发布产物:"
	@ls -lh $(RELEASE_DIR)/
	@cd $(RELEASE_DIR) && sha256sum * > checksums.txt && echo "📋 checksums.txt"

help:
	@echo "用法: make [目标]"
	@echo ""
	@echo "目标:"
	@echo "  make build      构建单二进制 (~2MB，静态+UPX，可直接运行)"
	@echo "  make run        构建并运行"
	@echo "  make run-dev    开发模式运行 (-data ../data)"
	@echo "  make test       运行测试"
	@echo "  make bench      性能测试 (7天)"
	@echo "  make bench-all  性能测试 (全部)"
	@echo "  make release    多平台发布包 (GitHub Release 用)"
	@echo "  make clean      清理构建产物"
	@echo "  make help       显示帮助"
	@echo ""
	@echo "参数:"
	@echo "  -data <path>  数据目录 (默认: ./data)"
	@echo "  -addr <addr>  监听地址 (默认: :8080)"
