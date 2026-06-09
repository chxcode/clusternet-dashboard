FROM node:22-alpine AS web-builder
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
ARG BASE_PATH=/clusternet
ENV BASE_PATH=${BASE_PATH}
RUN npm run build

FROM golang:1.26-alpine AS server-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/clusternet-dashboard ./cmd/clusternet-dashboard

FROM alpine:3.22
WORKDIR /app
RUN adduser -D -H -u 65532 dashboard
COPY --from=server-builder /out/clusternet-dashboard /app/clusternet-dashboard
COPY --from=web-builder /src/web/dist /app/web/dist
ENV PORT=8080
ENV BASE_PATH=/clusternet
ENV STATIC_DIR=/app/web/dist
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/clusternet-dashboard"]
