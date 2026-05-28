FROM golang:1.22-alpine AS builder
WORKDIR /app

RUN apk add --no-cache git

COPY services/control-plane/go.mod services/control-plane/go.sum ./
RUN go mod download

COPY services/control-plane/ .
COPY platform-libraries/ ./internal/platformlibs/files/
RUN CGO_ENABLED=0 GOOS=linux go build -o /velane ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /velane /velane
EXPOSE 8080
ENTRYPOINT ["/velane"]
