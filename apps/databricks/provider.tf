provider "azurerm" {
  features {}
}

provider "databricks" {
  host     = azurerm_databricks_workspace.this.workspace_url
  token    = "<databricks-token>"  # Consider using sensitive variables or secrets here
}