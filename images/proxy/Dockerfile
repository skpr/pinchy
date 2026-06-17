FROM golang:1.26-alpine AS build

WORKDIR /app

# Download deps first for layer caching.
COPY go.mod go.sum ./
RUN go mod download

# Source.
COPY . .

# Static binary.
RUN CGO_ENABLED=0 go build \
    -ldflags="-extldflags '-static'" \
    -o /pinchy-proxy \
    github.com/skpr/pinchy/cmd/pinchy-proxy

FROM alpine:3.23

RUN apk add --no-cache ca-certificates

COPY --from=build /pinchy-proxy /usr/local/bin/pinchy-proxy

EXPOSE 8080 3000

ENTRYPOINT ["pinchy-proxy"]
CMD ["listen"]
