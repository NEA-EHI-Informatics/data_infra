# Introduction

---


<!-- mtoc-start -->

* [Installation](#installation)
  * [Overrides](#overrides)
  * [Tailscale](#tailscale)
* [Dependencies](#dependencies)
* [PostgreSQL](#postgresql)
  * [Connection Passwords](#connection-passwords)

<!-- mtoc-end -->

# Installation

You'll need to set a secure key for the superset app. It's generated by `openssl rand -base64 42`.

We do not include the key in the argocd app.

## Overrides

There's a section in `values.yaml`: `configOverrides.<give a name>` which you can declare variables in `superset_config.py`. THese will override the default

## Tailscale

We added a tailscale sidecar to the `supersetNode` so we can access superset directly via the tailscale pod. (the other methods eg. service discovery and subnet router doesnt work with argocd installations).

# Dependencies

The superset chart depends on redis and postgresql. Connection string can be coded in the values.yaml file but a better way is to have this stored in a secret object.

# PostgreSQL

Two users are configured for connection to superset's postgres: `postgres` (admin) and `superset` (normal user)

## Connection Passwords

* `postgres` (admin)

  ```
  kubectl get secret superset-postgresql -o jsonpath='{.data.postgres-password}' | base64 --decode
  ```

* `superset`, the default password is `superset`

  ```
  kubectl get secret superset-postgresql -o jsonpath='{.data.password}' | base64 --decode
  ```
