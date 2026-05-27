FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/mcp-server ./cmd/server

FROM gcr.io/distroless/static-debian12

COPY --from=builder /out/mcp-server /mcp-server

EXPOSE 8090

ENTRYPOINT ["/mcp-server"]
