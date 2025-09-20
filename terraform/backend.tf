terraform {
  backend "azurerm" {
    resource_group_name  = "demo"
    storage_account_name = "demobd1"
    container_name       = "tfstate"
    key                  = "aks-cluster.terraform.tfstate"
  }
}
