FROM alpine:3.23

RUN apk add --no-cache ca-certificates

COPY pinchy-proxy /usr/local/bin/pinchy-proxy

EXPOSE 8080 3000

ENTRYPOINT ["pinchy-proxy"]
CMD ["listen"]
