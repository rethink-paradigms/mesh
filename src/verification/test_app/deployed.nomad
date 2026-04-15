variable "app_name" {
  type = string
}

variable "image" {
  type = string
}

variable "image_tag" {
  type = string
}

variable "count" {
  type    = number
  default = 1
}

variable "port" {
  type    = number
  default = 8080
}

variable "region" {
  type    = string
  default = "global"
}

variable "datacenter" {
  type    = string
  default = "dc1"
}

job "marketing-site" {
  region      = var.region
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
        "traefik.http.routers.${var.app_name}.rule=Host(`${var.app_name}.localhost`)",
        "traefik.consulcatalog.connect=false"
      ]

      check {
        type     = "http"
        path     = "/"
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

      # Secrets Injection via Nomad Native Variables
      template {
        data = <<EOF
{{ with nomadVar "jobs/${var.app_name}/secrets" }}
{{ range $k, $v := . }}
{{ $k }}="{{ $v }}"
{{ end }}
{{ end }}
EOF
        destination = "secrets/env"
        env         = true
      }

      resources {
        cpu    = 100
        memory = 128
      }
    }
  }
}
