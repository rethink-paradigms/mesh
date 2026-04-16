variable "acme_email" {
  type = string
  description = "Email for Let's Encrypt certificate notifications"
}

variable "acme_ca_server" {
  type    = string
  default = "https://acme-v02.api.letsencrypt.org/directory"
  description = "ACME CA server URL (default: Let's Encrypt production)"
}

variable "acme_storage_path" {
  type    = string
  default = "/letsencrypt/acme.json"
  description = "Path to store ACME certificates"
}

variable "use_tls_challenge" {
  type    = bool
  default = true
  description = "Use TLS-ALPN-01 challenge for certificate validation"
}

variable "use_http_challenge" {
  type    = bool
  default = false
  description = "Use HTTP-01 challenge as fallback"
}

variable "memory" {
  type    = number
  default = 256
  description = "Memory allocation in MB"
}

variable "cpu" {
  type    = number
  default = 200
  description = "CPU allocation in MHz"
}

variable "dashboard_enabled" {
  type    = bool
  default = true
  description = "Enable Traefik dashboard (insecure, set to false in production)"
}

variable "log_level" {
  type    = string
  default = "INFO"
  description = "Traefik log level (DEBUG, INFO, WARN, ERROR)"
}

variable "datacenter" {
  type    = string
  default = "dc1"
  description = "Consul datacenter name (e.g., dc1, dc-staging, dc-production)"
}

variable "traefik_image" {
  type    = string
  default = "traefik:v3.0"
  description = "Traefik Docker image with tag"
}

# ============================================================================
# Traefik Ingress Controller with Let's Encrypt
# ============================================================================
job "traefik" {
  type = "system"

  datacenters = [var.datacenter]

  # Run on all server nodes for high availability
  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  constraint {
    attribute = "${meta.role}"
    value     = "server"
  }

  group "traefik" {
    network {
      mode = "host"

      port "http" {
        static = 80
        to     = 80
      }

      port "https" {
        static = 443
        to     = 443
      }

      port "traefik" {
        static = 8080
        to     = 8080
      }
    }

    volume "acme" {
      type            = "host"
      read_only       = false
      source          = "traefik-acme"
    }

    service {
      name = "traefik"
      port = "https"

      tags = [
        "traefik.enable=true",
        "traefik.http.routers.api.rule=Host(`traefik.${NOMAD_REGION_east_us}.nomad`)",
        "traefik.http.routers.api.entrypoints=https",
        "traefik.http.routers.api.service=api@internal",
        "traefik.http.routers.api.tls=true"
      ]

      check {
        type     = "tcp"
        port     = "https"
        interval = "10s"
        timeout  = "2s"
      }

      # Connect to Consul for service discovery
      check {
        type     = "script"
        command  = "/bin/sh"
        args     = ["-c", "curl -s http://127.0.0.1:8080/ping"]
        interval = "30s"
        timeout  = "5s"
      }
    }

    task "traefik" {
      driver = "docker"

      config {
        image = var.traefik_image

        mount {
          type   = "volume"
          target = "/letsencrypt"
          source = "acme"
        }

        ports = ["http", "https", "traefik"]
      }

      template {
        data = <<EOF
[log]
  level = "${var.log_level}"

[api]
  dashboard = ${var.dashboard_enabled}
  insecure  = false  # Require authentication in production

[entryPoints]
  [entryPoints.web]
    address = ":80"
    [entryPoints.web.http.redirections]
      [entryPoints.web.http.redirections.entryPoint]
        to = "websecure"
        scheme = "https"

  [entryPoints.websecure]
    address = ":443"
    [entryPoints.websecure.http.tls]
      certResolver = "letsencrypt"

[certificatesResolvers.letsencrypt.acme]
  email = "${var.acme_email}"
  storage = "${var.acme_storage_path}"
  caServer = "${var.acme_ca_server}"

%{if var.use_tls_challenge}
  [certificatesResolvers.letsencrypt.acme.tlsChallenge]
%{end}

%{if var.use_http_challenge}
  [certificatesResolvers.letsencrypt.acme.httpChallenge]
    entryPoint = "web"
%{end}

[providers.consulCatalog]
  prefix = "traefik"
  exposedByDefault = false

  [providers.consulCatalog.endpoint]
    scheme = "http"
    address = "127.0.0.1:8500"

[providers.file]
  filename = "/etc/traefik/dynamic.toml"
EOF

        destination = "etc/traefik/traefik.toml"

        change_mode   = "restart"
        change_signal = "SIGHUP"
      }

      # Dynamic configuration for HTTP → HTTPS redirect middleware
      template {
        data = <<EOF
[http.middlewares.redirect-http-to-https.redirectScheme]
  scheme = "https"
  permanent = true
EOF

        destination = "etc/traefik/dynamic.toml"

        change_mode   = "restart"
        change_signal = "SIGHUP"
      }

      resources {
        cpu    = var.cpu
        memory = var.memory
      }
    }
  }
}
