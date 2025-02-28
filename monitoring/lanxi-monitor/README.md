# Introduction

`lanxi` is a lightweight http server targeted to be run on Raspberry PI. it has two endpoints `/metrics` and `/health`. The former exposes metrics for grafana alloy to scrape to push to prometheus.

# Build

To build the binary, run `go build`

```bash
GOOS=linux GOARCH=arm64 go build -o bin/lanxi-monitor .
```

# Installation

Copy the generated binary `lanxi-monitor` to the following directory `/home/ubuntu/lanxi/bin`, on PI attached to the laxi module.

# Alloy (Metrics Scraper)

For metrics to be pushed to grafana, there needs to be a metrics scraper and a prometheus instance. Alloy is a lightweight scraper that can be run on the same machine as the server. It will scrape the metrics and push them to a prometheus instance.

1. Set up service unit (`.service`) to run alloy.

    ```bash
    sudo dpkg -i alloy-1.6.1-1.arm64.deb
    ```

   > to confirm run `systemctl list-units --type=service | grep alloy` 

2. Set up the config see `monitoring/alloy/config.alloy`. the object below includes scraping `/metrics` from `lanxi-monitor`

    ```river
    prometheus.scrape "lanxi_monitor" {
    targets = [
        {
        job         = "lanxi-monitor",
        __address__ = "localhost:8080",
        },
    ]
    forward_to = [prometheus.remote_write.metrics_service.receiver]
    }
    ```
