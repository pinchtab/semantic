FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /usr/local/bin/semantic ./cmd/semantic

FROM alpine:3.21
RUN apk add --no-cache bash jq
COPY --from=builder /usr/local/bin/semantic /usr/local/bin/semantic
COPY tests/e2e/ /e2e/
COPY tests/e2e/assets/ /testdata/
RUN chmod +x /e2e/run.sh /e2e/lib.sh 2>/dev/null || true
ENTRYPOINT ["/bin/bash", "/e2e/run.sh"]
