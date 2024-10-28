# Blob Storage Account
resource "azurerm_storage_account" "blob_storage" {
  name                     = "nea-ehi-blobstorage"
  resource_group_name      = azurerm_resource_group.storage_rg.name
  location                 = azurerm_resource_group.storage_rg.location
  account_tier             = "Standard"
  account_replication_type = "LRS"

  blob_properties {
    versioning_enabled = true
  }

  tags = merge(
	local.tags,
	{
    	division = "stg"
  	}
  )

# Blob Container
resource "azurerm_storage_container" "blob_container" {
  name                  = "EETD"
  storage_account_name  = azurerm_storage_account.blob_storage.name
  container_access_type = "private"
}