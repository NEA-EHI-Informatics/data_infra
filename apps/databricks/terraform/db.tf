resource "azurerm_databricks_workspace" "mmed-workspaces" {
  name                       = "mmed"
  resource_group_name        = azurerm_resource_group.this.name
  location                   = var.region
  sku                        = "trial"
  managed_resource_group_name = azurerm_resource_group.this.name
  tags = merge(
    local.tags,
    {
      Division = "mmed"
    }
  )
  custom_parameters {
    no_public_ip                                         = var.no_public_ip
    virtual_network_id                                   = azurerm_virtual_network.this.id
    private_subnet_name                                  = azurerm_subnet.private.name
    public_subnet_name                                   = azurerm_subnet.public.name
    public_subnet_network_security_group_association_id  = azurerm_subnet_network_security_group_association.public.id
    private_subnet_network_security_group_association_id = azurerm_subnet_network_security_group_association.private.id
  }
}

resource "azurerm_databricks_workspace" "eetd-workspaces" {
  name                       = "eetd"
  resource_group_name        = azurerm_resource_group.this.name
  location                   = var.region
  sku                        = "trial"
  managed_resource_group_name = azurerm_resource_group.this.name
  tags = merge(
    local.tags,
    {
      Division = "eetd"
    }
  )
  custom_parameters {
    no_public_ip                                         = var.no_public_ip
    virtual_network_id                                   = azurerm_virtual_network.this.id
    private_subnet_name                                  = azurerm_subnet.private.name
    public_subnet_name                                   = azurerm_subnet.public.name
    public_subnet_network_security_group_association_id  = azurerm_subnet_network_security_group_association.public.id
    private_subnet_network_security_group_association_id = azurerm_subnet_network_security_group_association.private.id
  }
}

resource "azurerm_resource_group" "this" {
  name     = "${local.prefix}-rg"
  location = var.region
  tags = merge(
    local.tags,
  )
}

resource "databricks_storage_credential" "storage_credential" {
  name = "my_blob_storage_credential"

  azurerm_managed_identity {
    client_id = azurerm_user_assigned_identity.databricks_identity.client_id
  }
}

resource "databricks_external_location" "external_location" {
  name                    = "my_blob_external_location"
  storage_credential_name = databricks_storage_credential.storage_credential.name
  url                     = "wasbs://${azurerm_storage_container.blob_container.name}@${azurerm_storage_account.blob_storage.name}.blob.core.windows.net/"
  comment                 = "External location for flat files in Azure Blob Storage"
}

resource "databricks_volume" "my_blob_volume" {
  name              = "my_blob_volume"
  external_location = databricks_external_location.external_location.name
  comment           = "Volume for storing flat files in Azure Blob Storage"
}