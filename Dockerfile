FROM alpine:3.18.0@sha256:02bb6f428431fbc2809c5d1b41eab5a68350194fb508869a33cb1af4444c9b11

WORKDIR /opt/gh-jira-issue-sync

RUN apk update --no-cache && apk add ca-certificates

COPY bin/gh-jira-issue-sync /opt/gh-jira-issue-sync/gh-jira-issue-sync

COPY config.json /opt/gh-jira-issue-sync/config.json

ENTRYPOINT ["./gh-jira-issue-sync"]

CMD ["--config", "config.json"]
