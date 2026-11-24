FROM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/monetization ./cmd/monetization

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D monetization

WORKDIR /app

COPY --from=builder --chown=monetization:monetization /out/monetization /app/monetization

USER monetization
ENTRYPOINT ["/app/monetization"]
