.PHONY: dev build run clean test migrate

# 开发模式
dev:
	GIN_MODE=debug DEV_MODE=true go run cmd/server/main.go

# 构建
build:
	go build -o bin/regplatform cmd/server/main.go

# 运行构建产物
run: build
	./bin/regplatform

# 前端开发
web-dev:
	cd web && npm run dev

# 前端构建
web-build:
	cd web && npm run build

# 全量构建（后端 + 前端）
all: web-build build

# 清理
clean:
	rm -rf bin/
	rm -rf web/dist/

# 测试
test:
	go test ./...

# Docker
docker:
	docker-compose up -d --build

docker-down:
	docker-compose down

# Go 依赖
tidy:
	go mod tidy
