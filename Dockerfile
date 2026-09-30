FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /app/server ./cmd/server

FROM alpine:3.20

COPY --from=builder /app/server /app/server

ENTRYPOINT ["/app/server"]