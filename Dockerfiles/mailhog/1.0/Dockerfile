FROM alpine:3.16

USER root

RUN apk --no-cache add ca-certificates \
  && apk --no-cache add --virtual build-dependencies go git \
  && mkdir -p /root/gocode \
  && export GOPATH=/root/gocode \
  && go install github.com/mailhog/MailHog@latest \
  && mv /root/gocode/bin/MailHog /usr/local/bin \
  && rm -rf /root/gocode \
  && apk del --purge build-dependencies

RUN adduser -D -u 1000 mailhog &> /dev/null; exit 0
USER mailhog
WORKDIR /home/mailhog

ENTRYPOINT ["MailHog"]