FROM alpine:3.23

RUN apk add --no-cache ca-certificates

COPY pinchy-api /usr/local/bin/pinchy-api

EXPOSE 50051

ENTRYPOINT ["pinchy-api"]
CMD ["listen"]
