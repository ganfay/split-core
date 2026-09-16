FROM golang:1.26.2-alpine AS builder

WORKDIR /src

COPY proto/ ./proto/

COPY notifier/go.mod notifier/go.sum ./notifier/

WORKDIR /src/notifier

RUN go mod download

COPY notifier/ .

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/notify ./cmd/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/notify .

CMD ["./notify"]