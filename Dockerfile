FROM cgr.dev/chainguard/go:1.20@sha256:a50b9f354ad5a645e15b2bf97ac1153e650e7a0eec46cb014875019db1081a4d as build

COPY . . 
RUN CGO_ENABLED=0 go build .

FROM cgr.dev/chainguard/static

COPY --from=build gh-jira-issue-sync /bin/gh-jira-issue-sync

COPY config.json /etc/config.json

ENTRYPOINT ["/bin/gh-jira-issue-sync"]

CMD ["--config", "/etc/config.json"]
