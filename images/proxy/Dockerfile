FROM golang:1.26-alpine AS build

WORKDIR /app

# Module + vendored deps first for layer caching.
COPY go.mod go.sum ./
COPY vendor/ vendor/

# Source.
COPY . .

# Static binary (matches mise.toml: CGO_ENABLED=0, static link, vendored).
RUN CGO_ENABLED=0 go build -mod=vendor \
    -ldflags="-extldflags '-static'" \
    -o /pinchy-proxy \
    github.com/skpr/pinchy/cmd/pinchy-proxy

FROM alpine:3.23

RUN apk add --no-cache ca-certificates

COPY --from=build /pinchy-proxy /usr/local/bin/pinchy-proxy

EXPOSE 8080 3000

ENTRYPOINT ["pinchy-proxy"]
CMD ["listen"]
