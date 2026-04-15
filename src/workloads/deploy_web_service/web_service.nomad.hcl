variable "app_name" {
  type = string
  description = "The name of the application."
}

variable "image" {
  type = string
  description = "The Docker image to run."
}

variable "image_tag" {
  type = string
  default = "latest"
  description = "The tag of the Docker image."
}

variable "count" {
  type    = number
  default = 1
  description = "The number of instances to run."
}

variable "port" {
  type    = number
  default = 80
  description = "The port the application listens on."
}

variable "host_rule" {
  type = string
  description = "The Traefik host rule for routing traffic to the application."
}

variable "cpu" {
  type = number
  default = 100
  description = "The CPU allocation in MHz."
}

variable "memory" {
  type = number
  default = 128
  description = "The memory allocation in MB."
}

variable "datacenter" {
  type    = string
  default = "dc1"
  description = "Consul datacenter name (e.g., dc1, dc-staging, dc-production)"
}

variable "domain" {
  type    = string
  default = "localhost"
  description = "Base domain for Traefik routing (e.g., localhost, example.com, app.example.com)"
}

job "${var.app_name}" {
  datacenters = [var.datacenter]
  type        = "service"

  group "app" {
    count = var.count

    network {
      mode = "bridge"
      port "http" {
        to = var.port
      }
    }

    service {
      name = var.app_name
      port = "http"

      tags = [
        "traefik.enable=true",
        "traefik.http.routers.${var.app_name}.rule=Host(`${var.host_rule}`)"
      ]

      check {
        type     = "http"
        path     = "/health"
        interval = "10s"
        timeout  = "2s"
      }
    }

    task "server" {
      driver = "docker"

      config {
        image = "${var.image}:${var.image_tag}"
        ports = ["http"]
      }

      template {
        data = <<EOH
          {{ with nomadVar (printf "nomad/jobs/%s" .JOB) }}
          {{ range $key, $value := .Items }}
          {{ $key }}={{ $value }}
          {{ end }}
          {{ end }}
        EOH
        destination = "secrets/app.env"
        perms       = "0600"
      }

      env {
        PORT = "${NOMAD_PORT_http}"
        # Point to secrets file location for applications using file-based secrets
        SECRETS_FILE = "secrets/app.env"
      }

      resources {
        cpu    = var.cpu
        memory = var.memory
      }
    }
  }
}
