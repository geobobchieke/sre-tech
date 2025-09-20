output "cluster_name" {
  description = "The name of the created AKS cluster."
  value       = azurerm_kubernetes_cluster.aks.name
}