FROM golang:1.26.2-alpine AS builder

WORKDIR /src

COPY proto/ ./proto/

COPY split-notify/go.mod split-notify/go.sum ./split-notify/

WORKDIR /src/split-notify

RUN go mod download

COPY split-notify/ .

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/notify ./cmd/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/notify .

CMD ["./notify"]