# data_infra

All neccessary tooling required for data infra with Apache Superset (Visualisation), Trino and Unity Catalog, Apache Iceberg (Using Iceberg)



# Introduction

---

This is a self contained tutorial to set up the necessary data infrastructure on a K8S cluster. For the purposes of the this guide we will be using `microk8s`.

<!-- mtoc-start -->

* [Prerequisites](#prerequisites)

<!-- mtoc-end -->

# Prerequisites

You'll require a K8S cluster. This will be where your data infra will be deployed.

# Image repository 

We've chosen to store the docker container images on github container registry. You'll have to provide k8s with the login credentials

```
USERNAME=nea-ehi-informatics
PASSWORD=<CREDENTIAL>  # with `package:write`, `package:read`
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=$USERNAME \
  --docker-password=$PASSWORD
```
