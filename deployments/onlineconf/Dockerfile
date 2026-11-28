FROM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/onlineconf ./cmd/onlineconf

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D onlineconf

WORKDIR /app

COPY --from=builder --chown=onlineconf:onlineconf /out/onlineconf /app/onlineconf

USER onlineconf
ENTRYPOINT ["/app/onlineconf"]
