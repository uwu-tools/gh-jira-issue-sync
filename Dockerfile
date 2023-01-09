FROM alpine:3.17@sha256:f271e74b17ced29b915d351685fd4644785c6d1559dd1f2d4189a5e851ef753a

WORKDIR /opt/gh-jira-issue-sync

RUN apk update --no-cache && apk add ca-certificates

COPY bin/gh-jira-issue-sync /opt/gh-jira-issue-sync/gh-jira-issue-sync

COPY config.json /opt/gh-jira-issue-sync/config.json

ENTRYPOINT ["./gh-jira-issue-sync"]

CMD ["--config", "config.json"]
