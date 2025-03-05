<!-- vscode-markdown-toc -->
* 1. [Configuration](#Configuration)

<!-- vscode-markdown-toc-config
	numbering=true
	autoSave=true
	/vscode-markdown-toc-config -->
<!-- /vscode-markdown-toc -->

# Introduction

`lanxi` is a lightweight http server targeted to be run on Raspberry PI. it has two endpoints `/metrics` and `/health`. The former exposes metrics for grafana alloy to scrape to push to prometheus.



# Build

To build the binary, run `go build`

```bash
GOOS=linux GOARCH=arm64 go build -o bin/lanxi-monitor .
```

# Installation

Copy the generated binary `lanxi-monitor` to the following directory `/home/ubuntu/lanxi/bin`, on PI attached to the laxi module.

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

##  1. <a name='Configuration'></a>Configuration 
