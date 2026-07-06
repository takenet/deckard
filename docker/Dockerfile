FROM golang:1.26.4-alpine AS builder

WORKDIR /src

# Build cache dependencies first.
COPY go.mod go.sum ./
RUN go mod download

# Build binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/deckard ./internal/cmd/deckard

FROM alpine:3.22
RUN adduser -D -u 10001 deckard
USER deckard
WORKDIR /home/deckard
COPY --from=builder /out/deckard /usr/local/bin/deckard
EXPOSE 8081 8082
ENTRYPOINT ["/usr/local/bin/deckard"]
