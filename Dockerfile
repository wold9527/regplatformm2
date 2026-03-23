# 多阶段构建
# Stage 1: 前端构建
FROM node:20-alpine AS web-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: 后端构建
FROM golang:1.25-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /app/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -o regplatform cmd/server/main.go

# Stage 3: 运行镜像
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata chromium
WORKDIR /app
COPY --from=go-builder /app/regplatform .
COPY --from=go-builder /app/web/dist ./web/dist
COPY .env.example .env

ENV TZ=Asia/Shanghai
ENV CHROME_PATH=/usr/bin/chromium-browser

EXPOSE 8000
CMD ["./regplatform"]
