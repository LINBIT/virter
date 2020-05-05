FROM golang:alpine as builder

RUN apk update \
	&& apk add make git \
	&& rm -rf /var/cache/apk/*

WORKDIR /go/src/virter
COPY . .

RUN make && mv ./virter / && mv ./assets /

FROM alpine:latest

COPY --from=builder /virter /opt/virter/
COPY --from=builder /assets /opt/virter/assets

RUN apk update \
	&& apk add rsync \
	&& rm -rf /var/cache/apk/*

WORKDIR /opt/virter
CMD ["-h"]
ENTRYPOINT ["/opt/virter/virter"]
