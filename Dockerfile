FROM alpine:3.17@sha256:8914eb54f968791faf6a8638949e480fef81e697984fba772b3976835194c6d4

WORKDIR /opt/gh-jira-issue-sync

RUN apk update --no-cache && apk add ca-certificates

COPY bin/gh-jira-issue-sync /opt/gh-jira-issue-sync/gh-jira-issue-sync

COPY config.json /opt/gh-jira-issue-sync/config.json

ENTRYPOINT ["./gh-jira-issue-sync"]

CMD ["--config", "config.json"]
