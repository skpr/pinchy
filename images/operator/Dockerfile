FROM alpine:3.23

RUN apk add --no-cache ca-certificates

COPY pinchy-operator /usr/local/bin/pinchy-operator

ENTRYPOINT ["pinchy-operator"]
