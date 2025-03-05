# Introduction

![monitoring_diagram](https://github.com/user-attachments/assets/78abb27a-a6dc-4be0-ad35-b809a7602420)


# Alloy (Metrics Scraper)

For metrics to be pushed to grafana, a metrics scraper, in this case Granfana Alloy, is required. Alloy is a lightweight scraper that can be run on the same machine as where the LANXI client is running. It will scrape the `/metrics` endpoint according to the configuration and push them to a timeseries database in this case Grafana Mimir or Prometheus. The config can be found in the 

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
