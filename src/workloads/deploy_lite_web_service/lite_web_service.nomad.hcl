variable "app_name" {
  type = string
  description = "Application name"
}

variable "image" {
  type = string
  description = "Docker image"
}

variable "image_tag" {
  type    = string
  default = "latest"
  description = "Docker image tag"
}

variable "port" {
  type    = number
  default = 8080
  description = "Application port"
}

variable "domain" {
  type    = string
  default = ""
  description = "Domain for Caddy routing (leave empty for no domain)"
}

variable "cpu" {
  type    = number
  default = 100
  description = "CPU allocation in MHz"
}

variable "memory" {
  type    = number
  default = 128
  description = "Memory allocation in MB"
}

variable "datacenter" {
  type    = string
  default = "dc1"
  description = "Nomad datacenter name"
}

job "${var.app_name}" {
  datacenters = [var.datacenter]

  group "${var.app_name}" {
    network {
      mode = "host"
      port "http" {
        to = var.port
      }
    }

    task "${var.app_name}" {
      driver = "docker"

      config {
        image = "${var.image}:${var.image_tag}"
        ports = ["http"]
      }

      resources {
        cpu    = var.cpu
        memory = var.memory
      }
    }

    service {
      name = var.app_name
      port = "http"
    }
  }
}
