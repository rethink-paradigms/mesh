# Test Web Service - E2E Testing
# Simple web service used for multi-node E2E tests
job "test-web-service" {
  datacenters = [var.datacenter]
  type = "service"

  # Variable: Number of replicas (for multi-node scheduling tests)
  variable "count" {
    type    = number
    default = 1
    description = "Number of replicas to deploy"
  }

  # Variable: Unique job identifier (prevents conflicts)
  variable "job_id" {
    type    = string
    default = "default"
    description = "Unique identifier for this test run"
  }

  # Variable: Datacenter name
  variable "datacenter" {
    type    = string
    default = "dc1"
    description = "Consul datacenter name"
  }

  # Variable: Base domain for Traefik routing
  variable "domain" {
    type    = string
    default = "localhost"
    description = "Base domain for Traefik routing (e.g., localhost, example.com)"
  }

  # Use job_id to create unique job name
  name = "test-web-service-${var.job_id}"

  # Spread allocations across nodes for multi-node testing
  spread {
    attribute = "${node.unique.name}"
    weight    = 100
  }

  # Update strategy
  update {
    max_parallel = 1
    min_healthy_time = "10s"
    healthy_deadline = "3m"
    progress_deadline = "10m"
    auto_revert = false
  }

  group "web" {
    count = var.count

    network {
      mode = "bridge"

      port "http" {
        to = 80
      }
    }

    service {
      name = "test-web-service"
      port = "http"

      tags = [
        "traefik.enable=true",
        "traefik.http.routers.test-web-service.rule=Host(`test-web-service.${var.domain}`)",
        "test-e2e"
      ]

      check {
        type     = "http"
        path     = "/"
        interval = "10s"
        timeout  = "2s"
        port     = "http"
      }

      connect {
        sidecar_service {}
      }
    }

    task "server" {
      driver = "docker"

      config {
        image = "nginx:alpine"

        ports = ["http"]
      }

      resources {
        cpu    = 100
        memory = 64
      }

      # Custom response for content verification
      template {
        data = <<-EOF
          <!DOCTYPE html>
          <html>
          <head><title>E2E Test Service</title></head>
          <body>
            <h1>E2E Test Web Service</h1>
            <p>Job ID: {{ env "NOMAD_JOB_NAME" }}</p>
            <p>Allocation: {{ env "NOMAD_ALLOC_ID" }}</p>
            <p>Node: {{ env "node.unique.name" }}</p>
            <p>Datacenter: {{ env "NOMAD_DC_NAME" }}</p>
          </body>
          </html>
        EOF

        destination = "local/index.html"

        # Override default nginx page (not directly supported in nginx:alpine)
        # For actual E2E tests, we'd use a custom image or volume mount
        change_mode   = "noop"
      }

      log_sink {
        type = "file"
        config = {
          "file_name" = "stdout"
        }
      }
    }
  }
}
