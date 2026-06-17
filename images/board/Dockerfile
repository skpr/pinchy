FROM alpine:3.23

RUN apk add --no-cache ca-certificates

COPY pinchy-board /usr/local/bin/pinchy-board

EXPOSE 8090

ENTRYPOINT ["pinchy-board"]
CMD ["listen"]
