FROM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/identity ./cmd/identity

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D identity

WORKDIR /app

COPY --from=builder --chown=identity:identity /out/identity /app/identity

USER identity
ENTRYPOINT ["/app/identity"]
