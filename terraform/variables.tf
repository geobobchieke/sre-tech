variable "resource_group_name" {
  description = "The name of the resource group in which to create the cluster."
  type        = string
  default     = ""
}

variable "location" {
  description = "The Azure region where the resources will be created. Must support Availability Zones."
  type        = string
  default     = ""
}

variable "cluster_name" {
  description = "The name for the AKS cluster."
  type        = string
  default     = ""
}

variable "dns_prefix" {
  description = "The DNS prefix for the AKS cluster."
  type        = string
  default     = ""
}

variable "kubernetes_version" {
  description = "The version of Kubernetes to use for the cluster."
  type        = string
  default     = ""
}

variable "node_count" {
  description = "The initial number of nodes for the default node pool."
  type        = number
  default     = 1
}

variable "vm_size" {
  description = "The size of the virtual machines to use for the nodes."
  type        = string
  default     = ""
}

variable "vnet_address_space" {
  description = "The address space for the virtual network."
  type        = list(string)
  default     = ["10.20.0.0/16"]
}

variable "aks_subnet_address_space" {
  description = "The address space for the AKS subnet."
  type        = list(string)
  default     = ["10.20.1.0/24"]
}
