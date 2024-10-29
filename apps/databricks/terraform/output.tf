output "databricks_hosts" {
  value = {
    for name, workspace in azurerm_databricks_workspace.workspaces :
    name => "https://${workspace.workspace_url}/"
  }
}