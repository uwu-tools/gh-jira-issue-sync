FROM cgr.dev/chainguard/go:1.21@sha256:5b38eade1728ebe11473c832176e080e4baae756ef1f324e6712075b26bf111c as build

COPY . . 
RUN CGO_ENABLED=0 go build .

FROM cgr.dev/chainguard/static

COPY --from=build gh-jira-issue-sync /bin/gh-jira-issue-sync

COPY config.json /etc/config.json

ENTRYPOINT ["/bin/gh-jira-issue-sync"]

CMD ["--config", "/etc/config.json"]
