# Introduction

`lanxi` is a lightweight http server targeted to be run on Raspberry PI. it has two endpoints `/metrics` and `/health`. The former exposes metrics for grafana alloy to scrape to push to prometheus.

# Build

To build the binary, run `go build`

```bash
GOOS=linux GOARCH=arm64 go build -o bin/lanxi .
```
