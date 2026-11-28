FROM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/content ./cmd/content \
    && CGO_ENABLED=0 go build -o /out/reindex-posts ./cmd/reindex-posts

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D content

WORKDIR /app

COPY --from=builder --chown=content:content /out/content /app/content
COPY --from=builder --chown=content:content /out/reindex-posts /app/reindex-posts

USER content
ENTRYPOINT ["/app/content"]
