FROM alpine:latest

RUN apk --no-cache add openssh-client

COPY installer.sh /
COPY entry.sh /

CMD ["/entry.sh"]
