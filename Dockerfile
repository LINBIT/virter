FROM golang:alpine as builder

WORKDIR /go/src/virter
COPY . .

RUN go build && mv ./virter / && mv ./assets /

FROM alpine:latest

COPY --from=builder /virter /opt/virter/
COPY --from=builder /assets /opt/virter/assets

RUN apk update \
	&& apk add cdrkit rsync \
	&& rm -rf /var/cache/apk/*

WORKDIR /opt/virter
CMD ["-h"]
ENTRYPOINT ["/opt/virter/virter"]
