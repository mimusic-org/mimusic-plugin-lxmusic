BLUE=\033[0;34m
GREEN=\033[0;32m
NC=\033[0m

PLUGIN_NAME ?= lxmusic
VERSION ?= 1.0.0

.PHONY: help
help:
	@echo "$(BLUE)MiMusic 洛雪音源插件构建工具$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[0;32m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## 编译插件为 WASM 格式
	@echo "$(BLUE)正在构建 ${PLUGIN_NAME}.wasm...$(NC)"
	@rm -f ${PLUGIN_NAME}.wasm
	GOOS=wasip1 GOARCH=wasm go build -o ${PLUGIN_NAME}.wasm -buildmode=c-shared .
	@echo "$(GREEN)✓ 构建完成: ${PLUGIN_NAME}.wasm$(NC)"

.PHONY: clean
clean: ## 清理构建产物
	@echo "Cleaning build artifacts..."
	rm -f ${PLUGIN_NAME}.wasm
	@echo "Clean complete"

.PHONY: info
info: ## 显示插件信息
	@echo "$(BLUE)插件名称: ${PLUGIN_NAME}$(NC)"
	@echo "$(BLUE)版本: ${VERSION}$(NC)"
	@echo "$(BLUE)目标架构: WASIP1/WASM$(NC)"

all: build info
