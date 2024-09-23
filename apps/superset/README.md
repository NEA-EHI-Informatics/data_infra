# Introduction

---


<!-- mtoc-start -->

* [Dependencies](#dependencies)

<!-- mtoc-end -->

# Dependencies

The superset chart depends on redis and postgresql. Connection string can be coded in the values.yaml file but a better way is to have this stored in a secret object.

* Create connection secret

  ```bash
  microk8s kubectl create secret generic superset-db-secret \
    --from-literal=postgresql-password=superset-password \
    --from-literal=redis-password=redis-password \
    -n superset
  ```

