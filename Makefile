# ============================================================================
# PrivacyOS build system
# ============================================================================
SHELL := /bin/bash
ROOT  := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
VERSION := $(shell tr -d '[:space:]' < $(ROOT)/VERSION)
GO ?= go
NPM ?= npm
LDFLAGS := -s -w -X privacyos/backend/internal/version.Version=$(VERSION)

ISO_SOURCE ?= $(ROOT)/../开发文档/debian-13.7.0-amd64-DVD-1.iso
ISO_OUTPUT ?= $(ROOT)/build/privacyos-$(VERSION)-amd64.iso

.DEFAULT_GOAL := help
.PHONY: help all backend web test vet check payload iso iso-docker verify dev dev-backend dev-web demo demo-stop clean fmt

help:
	@echo "PrivacyOS build targets"
	@echo ""
	@echo "  make backend    编译后端到 build/privacyos-server"
	@echo "  make web        安装依赖并构建前端"
	@echo "  make all        backend + web"
	@echo "  make test       运行后端单元测试"
	@echo "  make check      go vet + 单测 + i18n 键检查 + 脚本语法检查"
	@echo "  make dev        启动开发环境（后端 8080 + Vite 5173）"
	@echo "  make demo       在后台启动桌面演示（http://127.0.0.1:8080）"
	@echo "  make demo-stop  停止后台演示"
	@echo "  make payload    构建 ISO 载荷（Linux 二进制 + 前端 + 安装资源）"
	@echo "  make iso        在 Linux 上构建 ISO（需 xorriso）"
	@echo "  make iso-docker 通过 Docker 构建 ISO（macOS 推荐）"
	@echo "  make verify     ISO=$(ISO_OUTPUT) 校验生成的 ISO"
	@echo "  make clean      清理构建产物"

build:
	@mkdir -p $(ROOT)/build

all: backend web

backend: build
	cd $(ROOT)/backend && CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" \
		-o $(ROOT)/build/privacyos-server ./cmd/privacyos-server
	@echo "backend -> build/privacyos-server ($(VERSION))"

web:
	cd $(ROOT)/frontend && if [ ! -d node_modules ]; then $(NPM) install --no-audit --no-fund; fi
	cd $(ROOT)/frontend && $(NPM) run build
	@echo "web -> frontend/dist"

test:
	cd $(ROOT)/backend && $(GO) test ./...

vet:
	cd $(ROOT)/backend && $(GO) vet ./...

check:
	cd $(ROOT)/backend && $(GO) vet ./... && $(GO) test ./...
	cd $(ROOT)/frontend && node scripts/check-i18n.mjs
	@set -e; for f in $(ROOT)/installer/scripts/*.sh $(ROOT)/installer/payload/*.sh $(ROOT)/scripts/*.sh; do \
		bash -n "$$f" || exit 1; done
	@echo "check: OK"

dev:
	$(ROOT)/scripts/dev.sh

demo:
	$(ROOT)/scripts/start-demo.sh --build

demo-stop:
	$(ROOT)/scripts/stop-demo.sh

payload:
	$(ROOT)/installer/scripts/build-payload.sh

iso: payload
	$(ROOT)/installer/scripts/build-iso.sh --source "$(ISO_SOURCE)" --output "$(ISO_OUTPUT)"

iso-docker: payload
	$(ROOT)/installer/scripts/build-in-docker.sh --source "$(ISO_SOURCE)" --output "$(ISO_OUTPUT)"

verify:
	$(ROOT)/installer/scripts/verify-iso.sh "$(ISO_OUTPUT)"

clean:
	rm -rf $(ROOT)/build $(ROOT)/frontend/dist
	@echo "cleaned"

fmt:
	cd $(ROOT)/backend && gofmt -w .
