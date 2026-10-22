FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /server ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /server /server
COPY configs/config.yaml /configs/config.yaml

EXPOSE 8080

CMD ["/server", "-config", "/configs/config.yaml"]
