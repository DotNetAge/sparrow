.PHONY: build run test clean deps fmt lint docker-build docker-run

# 构建二进制文件
build:
	go build -o bin/purchase-service cmd/main.go

# 运行服务
run:
	go run cmd/main.go

# 运行测试
test:
	go test ./...

# 清理构建文件
clean:
	rm -rf bin/

# 安装依赖
deps:
	go mod tidy
	go mod download

# 格式化代码
fmt:
	gofmt -s -w .

# 代码检查
lint:
	golangci-lint run

# 构建Docker镜像
docker-build:
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs); \
		echo "Building Docker image with $$APP_NAME:$$APP_VERSION"; \
		docker build -t $$APP_NAME:$$APP_VERSION .; \
	else \
		echo "Building Docker image with purchase-service:latest"; \
		docker build -t purchase-service:latest .; \
	fi

# 运行Docker容器
docker-run:
	docker-compose up --build -d

# 停止Docker容器
docker-stop:
	docker-compose down

# 查看Docker容器日志
docker-logs:
	docker-compose logs -f

# 重启Docker容器
docker-restart:
	docker-compose restart

# 开发环境运行
dev:
	go run cmd/main.go