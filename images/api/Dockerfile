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
    -o /pinchy-api \
    github.com/skpr/pinchy/cmd/pinchy-api

FROM alpine:3.23

RUN apk add --no-cache ca-certificates

COPY --from=build /pinchy-api /usr/local/bin/pinchy-api

EXPOSE 50051

ENTRYPOINT ["pinchy-api"]
CMD ["listen"]
