# Blog System Makefile

# 变量
APP_NAME = blog-system
MAIN_PATH = ./cmd/server
BUILD_PATH = ./build

# Go命令
GO = go
GOBUILD = $(GO) build
GORUN = $(GO) run
GOTEST = $(GO) test
GOMOD = $(GO) mod

# 颜色
GREEN = \033[0;32m
YELLOW = \033[0;33m
RED = \033[0;31m
NC = \033[0m # No Color

.PHONY: all build run test clean help

## all: 默认目标
all: build

## build: 编译项目
build:
	@echo "$(GREEN)正在编译项目...$(NC)"
	@mkdir -p $(BUILD_PATH)
	$(GOBUILD) -o $(BUILD_PATH)/$(APP_NAME) $(MAIN_PATH)
	@echo "$(GREEN)编译完成: $(BUILD_PATH)/$(APP_NAME)$(NC)"

## run: 运行项目
run:
	@echo "$(GREEN)正在启动服务器...$(NC)"
	$(GORUN) $(MAIN_PATH)

## test: 运行测试
test:
	@echo "$(GREEN)正在运行测试...$(NC)"
	$(GOTEST) -v ./...

## clean: 清理构建文件
clean:
	@echo "$(YELLOW)正在清理构建文件...$(NC)"
	@rm -rf $(BUILD_PATH)
	@rm -f blog.db
	@echo "$(GREEN)清理完成$(NC)"

## deps: 下载依赖
deps:
	@echo "$(GREEN)正在下载依赖...$(NC)"
	$(GOMOD) download
	@echo "$(GREEN)依赖下载完成$(NC)"

## tidy: 整理依赖
tidy:
	@echo "$(GREEN)正在整理依赖...$(NC)"
	$(GOMOD) tidy
	@echo "$(GREEN)依赖整理完成$(NC)"

## fmt: 格式化代码
fmt:
	@echo "$(GREEN)正在格式化代码...$(NC)"
	$(GO) fmt ./...
	@echo "$(GREEN)代码格式化完成$(NC)"

## vet: 代码检查
vet:
	@echo "$(GREEN)正在检查代码...$(NC)"
	$(GO) vet ./...
	@echo "$(GREEN)代码检查完成$(NC)"

## help: 显示帮助信息
help:
	@echo "$(GREEN)Blog System API Server$(NC)"
	@echo ""
	@echo "可用命令:"
	@echo "  make build   - 编译项目"
	@echo "  make run     - 运行项目"
	@echo "  make test    - 运行测试"
	@echo "  make clean   - 清理构建文件"
	@echo "  make deps    - 下载依赖"
	@echo "  make tidy    - 整理依赖"
	@echo "  make fmt     - 格式化代码"
	@echo "  make vet     - 代码检查"
	@echo "  make help    - 显示帮助信息"
