# Introduction

![monitoring_diagram](https://github.com/user-attachments/assets/78abb27a-a6dc-4be0-ad35-b809a7602420)


# Alloy (Metrics Scraper)

For metrics to be pushed to grafana, a metrics scraper, in this case Granfana Alloy, is required. Alloy is a lightweight scraper that can be run on the same machine as where the LANXI client is running. It scrapes `/metrics` and pushes telemetry data to a timeseries database (eg. Grafana Mimir or Prometheus). See [config](./alloy/config.alloy) for more information.


## Pre-requisites

`lanximonitor` must be running preferably as a `systemd` service unit. Have the binary placed at the path specified by `ExecStart`.

```bash
batcat --plain /etc/systemd/system/lanxi-monitor.service
[Unit]
Description=LANXI Monitor Service
After=network.target

[Service]
ExecStart=/home/ubuntu/lanxi/bin/lanxi-monitor
WorkingDirectory=/home/ubuntu/lanxi
Restart=always
User=ubuntu
Group=ubuntu
Environment="PORT=8080"
LimitNOFILE=4096

[Install]
WantedBy=multi-user.target
```

### Start and Enable lanxi-monitor

```bash
sudo systemctl start lanxi-monitor
sudo systemctl enable lanxi-monitor
```


1. Set up service unit (`.service`) to run alloy.

    ```bash
    # download this from the github release page
    sudo dpkg -i alloy-1.6.1-1.arm64.deb
    ```

   > to confirm run `systemctl list-units --type=service | grep alloy` 

2. Set up the config see `monitoring/alloy/config.alloy`. the object below includes scraping `/metrics` from `lanximonitor`. 

    * See example of config:

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

    * Restart systemd

      ```bash
      sudo systemclt daemon-reload
      ```
