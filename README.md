# AKS Go Application Demo

This demo demonstrates deploying a **Go application with PostgreSQL backend** on **Azure Kubernetes Service (AKS)**, using **Azure Container Registry (ACR)** for container images, **NGINX Ingress** for routing, **Cert-Manager** for TLS, and **Prometheus/Grafana** for monitoring.  

---

## Prerequisites

Before starting, ensure you have:

* [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli) installed and logged in  
* [Terraform](https://www.terraform.io/downloads) installed  
* [kubectl](https://kubernetes.io/docs/tasks/tools/) installed and configured  
* [Helm](https://helm.sh/docs/intro/install/) installed  
* Docker installed and running  

---

## Folder Structure

## Folder Structure

```text
.
├── k8s/                     # Kubernetes manifests for Go app & Postgres
│   ├── go-app-deployment.yaml
│   ├── go-app-service.yaml
│   ├── go-app-ingress.yaml
│   └── postgres-secret.yaml
├── postgres-exporter/        # Prometheus Postgres exporter manifests
│   └── postgres-exporter.yaml
├── helm_values/              # Custom Helm values
│   ├── nginx-values.yaml
│   └── kube-prometheus-stack-values.yaml
└── main.tf                   # Terraform infrastructure code

---

## Terraform Infrastructure

Terraform provisions:

* **Resource Group**  
* **Virtual Network & Subnet** for AKS  
* **AKS Cluster** with:
  * 1 system node pool
  * 1 workload node pool (autoscaling enabled, zones 1-3)  
* **NGINX Ingress Controller** using Helm  
* **Prometheus/Grafana Stack** using Helm  
* **Azure Container Registry (ACR)**  
* **Role Assignment** for AKS managed identity to pull images from ACR  

### Apply Terraform

```bash
terraform init
terraform plan
terraform apply

---

## Helm Deployments via Terraform

## Monitoring

The following components are deployed automatically by Terraform using Helm:

* **NGINX Ingress Controller** (`nginx-ingress` namespace)  
* **Prometheus & Grafana Stack** (`monitoring` namespace)

**Access These Services:**  
* All services are exposed via **LoadBalancer**.  
* To get the external IP:
kubectl get svc -n nginx-ingress

Access the app: 

http://<external-ip>

## Deploy postgres exporter

kubectl apply -f postgres-exporter/

# Docker

Building and Pushing Docker Images  

Tag the Docker image

```bash
docker tag task demoacrtest12.azurecr.io/demo/go-demo:v1

Push the image to ACR

az acr login --name demoacrtest12
docker push demoacrtest12.azurecr.io/demo/go-demo:v1

---
Note: Refer to the Deployment folder for all manifests

# TLS / HTTPS with Cert-Manager

## Install Cert-Manager

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml

##Create ClusterIssuer

kubectl apply -f cert-manager/

##Annotate Ingress

metadata:
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod

---

## Kubernetes Deployment Manifests

All manifests are stored in the `k8s/` folder:

* `go-app-deployment.yaml` → Deployment of Go application  
* `go-app-service.yaml` → Service exposing Go app internally  
* `go-app-ingress.yaml` → Ingress resource for routing & TLS  
* `postgres-secret.yaml` → Secret storing PostgreSQL connection string 

## Deploying the Go Application

**Apply manifests in go/k8s folder**

```bash
kubectl apply -f go/k8s/

Also hpa is enabled, run kubectl get hpa to fetch the current configs

# Notes

* Workload node pool uses **autoscaling**: min 1, max 3 nodes  
* Application container is pulled securely from ACR using **AKS Managed Identity + Role Assignment**  
* Temporary secrets may appear during certificate issuance; final TLS secret is created automatically  
* Postgres exporter collects metrics for Prometheus/Grafana dashboards
* demo.<nginx loadbalancer ip>.nip.io have used this host for demo, replace the nginx ip after deployment


