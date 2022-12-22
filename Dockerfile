FROM alpine:3.17

WORKDIR /opt/gh-jira-issue-sync

RUN apk update --no-cache && apk add ca-certificates

COPY bin/gh-jira-issue-sync /opt/gh-jira-issue-sync/gh-jira-issue-sync

COPY config.json /opt/gh-jira-issue-sync/config.json

ENTRYPOINT ["./gh-jira-issue-sync"]

CMD ["--config", "config.json"]
