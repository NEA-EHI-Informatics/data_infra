terraform {
  required_providers {
    azurerm = "~> 4.0"
    random  = "~> 3.6"
    databricks = {
      source = "databricks/databricks"
    }
  }
}

resource "random_string" "naming" {
  special = false
  upper   = false
  length  = 6
}

locals {
  prefix = "databricksdemo${random_string.naming.result}"
  tags = {
    Environment = "stg"
    Owner       = "nea-ehi"
  }
}