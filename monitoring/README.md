# Introduction

![monitoring drawio](https://github.com/user-attachments/assets/b53923fe-2ecf-409d-980a-6c6643d167ee)


In this folder, we store the necessary code for setting up a monitoring and alert system which comprises (1) **metrics scraper**, (2) **HTTP server** + **lanxi client** to interact with the HBK World sensor, (3) a **timeseries database** such as Prometheus or Mimir to store the telemetric data, (4) monitoring **dashboard** eg. Granfana and (5) an installable app on user's mobile device to alert of times when the readings are out of range. as part of the **Integrated Risk Management (IRM) client**

The `lanxiclient` starts a recording by sending a series of API calls to initiate a recording `open > create > configure > measure` and similarly closes a recording by `stop > finish > close`

<img src="https://private-user-images.githubusercontent.com/2868858/418487862-a77db5f4-9cf3-4891-94a6-2e07307cafc8.png?jwt=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJnaXRodWIuY29tIiwiYXVkIjoicmF3LmdpdGh1YnVzZXJjb250ZW50LmNvbSIsImtleSI6ImtleTUiLCJleHAiOjE3NDExNTY3MjUsIm5iZiI6MTc0MTE1NjQyNSwicGF0aCI6Ii8yODY4ODU4LzQxODQ4Nzg2Mi1hNzdkYjVmNC05Y2YzLTQ4OTEtOTRhNi0yZTA3MzA3Y2FmYzgucG5nP1gtQW16LUFsZ29yaXRobT1BV1M0LUhNQUMtU0hBMjU2JlgtQW16LUNyZWRlbnRpYWw9QUtJQVZDT0RZTFNBNTNQUUs0WkElMkYyMDI1MDMwNSUyRnVzLWVhc3QtMSUyRnMzJTJGYXdzNF9yZXF1ZXN0JlgtQW16LURhdGU9MjAyNTAzMDVUMDYzMzQ1WiZYLUFtei1FeHBpcmVzPTMwMCZYLUFtei1TaWduYXR1cmU9NzI2NzRkNmJjOTY5Y2ExOTNiNTNjMTdmNmU5ZTU4ODhmMzgxZTlhN2Q4NjY5OTZhMGRiNzU0YTM1NDZlODg0OCZYLUFtei1TaWduZWRIZWFkZXJzPWhvc3QifQ.ZTmW1KUGEjihyZOvnEY91f2Sl2on-SzLfd411d0wIc0" width="400">

# Table of Contents
<!-- vscode-markdown-toc -->
* 1. [Pre-requisites](#Pre-requisites)
	* 1.1. [Start and Enable lanxi-monitor](#StartandEnablelanxi-monitor)
* 2. [Installation](#Installation)

<!-- vscode-markdown-toc-config
	numbering=true
	autoSave=true
	/vscode-markdown-toc-config -->
<!-- /vscode-markdown-toc -->


# Networking

We set the hostname for the pi to follow the convention `ehi-<BRANCH>-<sensor_type>-<location><id>`

```bash
sudo hostnamectl set-hostname ehi-beb-accelpi-lab01
````

Because the sensor is connected to the PI via LAN, we would like the IP to the LANXI module to be fixed. The LANXI module defaults to 169.

1. Get lanxi MAC address

  * Run `arp` and get mac address of the attached LANXI module:

    ```bash
    $ arp -a

    _gateway (<IP>) at <MAC_ADDRESS> [ether] on wlan0
    ? (<IP>) at <MAC_ADDRESS> [ether] on wlan0
    ? (169.254.61.199) at <MAC_ADDRESS> [ether] on eth0 <- this one
    ```

2. Set IPs 

  * Install `dnsmasq`

    ```bash
    sudo apt update && sudo apt install dnsmasq -y
    ```

  * Edit config `/etc/dnsmasq.conf` 

    PI's `eth0` device will be assigned IPs in this range (`169.254.61.1` - `169.254.61.254`). Setting port=0 will disable DNS function, 
    we dont need it.

    ```
    port=0
    interface=eth0
    dhcp-range=169.254.61.1,169.254.61.254,255.255.0.0,infinite
    dhcp-host=<MAC_ADDRESS>,<DESIRED_IP>,infinite
    ```

# Alloy (Metrics Scraper)

For metrics to be pushed to grafana, a metrics scraper, in this case [Granfana Alloy](https://grafana.com/docs/alloy/latest/), is required. Alloy is a lightweight scraper on baremetal machine where `lanximonitor` is running. Alloy scrapes `/metrics` and pushes telemetry data to a timeseries database (eg. Grafana Mimir or Prometheus). See [config](./alloy/config.alloy) for more information.


##  1. <a name='Pre-requisites'></a>Pre-requisites

`lanximonitor` must be running preferably as a `systemd` service unit. 

To set up the service unit have this chunk placed in `/etc/systemd/system/lanxi-monitor.service`. `lanximonitor` should be found at the path specified by `ExecStart`.

```bash
[Unit]
Description=LANXI Monitor Service
After=network.target

[Service]
ExecStart=/home/ubuntu/lanxi/bin/lanximonitor
WorkingDirectory=/home/ubuntu/lanxi
Restart=always
User=ubuntu
Group=ubuntu
Environment="PORT=8080"
LimitNOFILE=4096

[Install]
WantedBy=multi-user.target
```

###  1.1. <a name='StartandEnablelanxi-monitor'></a>Start and Enable lanxi-monitor

```bash
sudo systemctl start lanxi-monitor
sudo systemctl enable lanxi-monitor
```

##  2. <a name='Installation'></a>Installation 


1. Set up service unit 

    ```bash
    # download .deb from the github release page
    curl -OL https://github.com/grafana/alloy/releases/download/v1.6.1/alloy-1.6.1-1.arm64.deb
    sudo dpkg -i alloy-1.6.1-1.arm64.deb
    ```

   Confirm by running: `systemctl list-units --type=service | grep alloy` 

2. Set up [config](./alloy/config.alloy). the object below includes scraping `/metrics` from `lanximonitor`. 

    * See example of config:

      ```river
      // default scrape interval is 1 minute
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

# Usage 

  ```bash
  ./lanximonitor --help
  Usage of ./lanxi-monitor-0.0.2:
    -deviceID string
          Device identifier (default "lanxi-01")
    -httpPort int
          Port of the HTTP server (default 8080)
    -lanxiConfig string
          LAN-XI configuration file (default "setup.json")
    -lanxiHost string
          IP of the LAN-XI module (default "169.254.61.199")
    -location string
          Device location (default "lab-1")
  ```
