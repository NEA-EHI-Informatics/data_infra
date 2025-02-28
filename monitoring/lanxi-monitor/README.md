# Introduction

`lanxi` is a lightweight http server targeted to be run on Raspberry PI. it has two endpoints `/metrics` and `/health`. The former exposes metrics for grafana alloy to scrape to push to prometheus.

# Build

To build the binary, run `go build`

```bash
GOOS=linux GOARCH=arm64 go build -o bin/lanxi-monitor .
```

# Installation

The recommended way of running `lanxi-monitor` is as a `systemd` service. 

This ensures that the service:  
- **Starts automatically** when the Raspberry Pi boots up.  
- **Restarts automatically** if it crashes or stops unexpectedly.  

The `systemd` service file should be found at: `/etc/systemd/system/lanxi-monitor.service`

1. Reload system daemon

  ```bash
  sudo systemctl daemon-reload
  ```

2. Start the service

  ```bash
  sudo systemctl start lanxi-monitor
  ```

3. Enable the service to start when the system boots up

  ```bash
  sudo systemctl enable lanxi-monitor
  ```

In the service definition, it points the binary `ExecStart=/home/ubuntu/lanxi/bin/lanxi-monitor`. Place the built binary `lanxi-monitor` in that directory `/home/ubuntu/lanxi/bin`.

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
