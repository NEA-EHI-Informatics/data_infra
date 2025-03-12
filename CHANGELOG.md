We keep track of major changes here. Because there's multiple apps, if there

# 2025-03-08

* Include `VERSION` and `CHANGELOG`
* Added client `lanxi` and HTTP server `lanximonitor` as part of **EHI lab monitoring**. `lanximonitor` starts as a HTTP server and `lanxi` client sends REST API requests to the module to start recording and processing TCP binary data stream.
* Include `.ksy` definition for HBK World's `openapi` binary data and generated (and modified) `.go` parser.
* Include systemd service `lanxi-monitor.service`
* Include example `setup.json` used to configure a recording
* Include `metrics.go` for prometheus metrics
