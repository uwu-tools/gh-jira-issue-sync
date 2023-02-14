FROM alpine:3.17.2@sha256:69665d02cb32192e52e07644d76bc6f25abeb5410edc1c7a81a10ba3f0efb90a

WORKDIR /opt/gh-jira-issue-sync

RUN apk update --no-cache && apk add ca-certificates

COPY bin/gh-jira-issue-sync /opt/gh-jira-issue-sync/gh-jira-issue-sync

COPY config.json /opt/gh-jira-issue-sync/config.json

ENTRYPOINT ["./gh-jira-issue-sync"]

CMD ["--config", "config.json"]
