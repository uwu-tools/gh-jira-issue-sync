FROM cgr.dev/chainguard/go:1.20@sha256:8864ff1cd54f5819063ea23133a9f03d7626cec7e9fba8efa7004c71176adc48 as build

COPY . . 
RUN CGO_ENABLED=0 go build .

FROM cgr.dev/chainguard/static

COPY --from=build gh-jira-issue-sync /bin/gh-jira-issue-sync

COPY config.json /etc/config.json

ENTRYPOINT ["/bin/gh-jira-issue-sync"]

CMD ["--config", "/etc/config.json"]
