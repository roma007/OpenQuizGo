SHELL := /bin/bash
-include .env
export

GO ?= go
BIN_DIR := bin

.PHONY: help tidy fmt vet test build run dev migrate-up migrate-down migrate-status migrate-version hooks bump

help: ## 显示可用命令
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "%-18s %s\n",$$1,$$2}'

tidy: ## 整理依赖
	$(GO) mod tidy

fmt: ## 格式化代码
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

vet: ## 静态检查
	$(GO) vet ./...

test: ## 运行测试
	$(GO) test -p 1 ./...

test-int: ## 集成测试（需 TEST_DATABASE_URL / TEST_REDIS_URL）
	$(GO) test -p 1 ./internal/identity ./internal/question ./internal/quiz ./internal/battle ./internal/dashboard ./internal/membership ./internal/topic ./internal/rule ./internal/watch ./internal/report -v

build: ## 编译二进制
	$(GO) build -trimpath -o $(BIN_DIR)/api ./cmd/api
	$(GO) build -trimpath -o $(BIN_DIR)/migrate ./cmd/migrate

run: ## 启动 API（需先迁移）
	$(GO) run ./cmd/api

migrate-up: ## 执行迁移
	$(GO) run ./cmd/migrate up

migrate-down: ## 回滚一个迁移
	$(GO) run ./cmd/migrate down

migrate-status: ## 查看迁移状态
	$(GO) run ./cmd/migrate status

migrate-version: ## 查看迁移版本
	$(GO) run ./cmd/migrate version

dev: migrate-up run ## 迁移并启动

hooks: ## 安装 git 钩子（每次提交自动升级版本 1.0.X）
	git config core.hooksPath scripts/git-hooks
	@echo "git hooks 已启用：scripts/git-hooks"

bump: ## 手动升级版本（1.0.X → 1.0.X+1）
	node scripts/bump-version.mjs
