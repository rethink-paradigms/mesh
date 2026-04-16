#!/bin/bash
set -e

# Pulumi injects these variables
TAILSCALE_KEY="{{ TAILSCALE_KEY }}"
LEADER_IP="{{ LEADER_IP }}"
ROLE="{{ ROLE }}"
HAS_GPU="{{ HAS_GPU }}"                           # "true" or "false"
CUDA_VERSION="{{ CUDA_VERSION }}"                 # e.g., "12.1"
DRIVER_VERSION="{{ DRIVER_VERSION }}"             # e.g., "535"
ENABLE_SPOT_HANDLING="{{ ENABLE_SPOT_HANDLING }}" # "true" or "false"
SPOT_CHECK_INTERVAL="{{ SPOT_CHECK_INTERVAL }}"   # e.g., "5"
SPOT_GRACE_PERIOD="{{ SPOT_GRACE_PERIOD }}"       # e.g., "90"
CLUSTER_TIER="{{ CLUSTER_TIER }}"
ENABLE_CADDY="{{ ENABLE_CADDY }}"

bash scripts/01-install-deps.sh

if [ "$CLUSTER_TIER" != "lite" ]; then
	bash scripts/02-install-tailscale.sh "$TAILSCALE_KEY"
fi

bash scripts/03-install-hashicorp.sh

# GPU-specific setup (only if HAS_GPU == "true")
if [ "$HAS_GPU" == "true" ]; then
	echo ">>> [04] Installing GPU drivers..."
	bash scripts/04-install-gpu-drivers.sh "$DRIVER_VERSION" "$CUDA_VERSION"

	echo ">>> [05] Installing nomad-device-nvidia plugin..."
	bash scripts/05-install-nvidia-plugin.sh
fi

if [ "$CLUSTER_TIER" != "lite" ]; then
	bash scripts/06-configure-consul.sh "$LEADER_IP" "$ROLE"
fi
bash scripts/07-configure-nomad.sh "$ROLE" "$HAS_GPU"

# GPU verification (only if HAS_GPU == "true")
if [ "$HAS_GPU" == "true" ]; then
	echo ">>> [08] Verifying GPU setup..."
	bash scripts/08-verify-gpu.sh
fi

# Spot instance interruption handling (only if ENABLE_SPOT_HANDLING == "true")
if [ "$ENABLE_SPOT_HANDLING" == "true" ]; then
	echo ">>> [09] Installing spot instance interruption handler..."

	# Create spot handler systemd service
	cat <<EOF >/etc/systemd/system/spot-handler.service
[Unit]
Description=AWS Spot Instance Interruption Handler
After=nomad-client.service
Requires=nomad-client.service

[Service]
Type=simple
Environment="SPOT_CHECK_INTERVAL=${SPOT_CHECK_INTERVAL:-5}"
Environment="SPOT_GRACE_PERIOD=${SPOT_GRACE_PERIOD:-90}"
Environment="NOMAD_ADDR=http://127.0.0.1:4646"
ExecStart=/opt/ops-platform/scripts/09-handle-spot-interruption.sh
Restart=on-failure
RestartSec=10s

[Install]
WantedBy=multi-user.target
EOF

	# Copy script to permanent location
	cp scripts/09-handle-spot-interruption.sh /opt/ops-platform/scripts/09-handle-spot-interruption.sh
	chmod +x /opt/ops-platform/scripts/09-handle-spot-interruption.sh

	# Enable and start spot handler service
	systemctl daemon-reload
	systemctl enable spot-handler.service
	systemctl start spot-handler.service

	echo ">>> [09] Spot handler installed and started"
fi

if [ "$ENABLE_CADDY" == "true" ]; then
	bash scripts/10-install-caddy.sh
	mkdir -p /opt/caddy/data
fi

if [ "$ENABLE_CADDY" == "true" ]; then
	if ! grep -q "caddy-data" /etc/nomad.d/nomad.hcl 2>/dev/null; then
		echo 'host_volume "caddy-data" {
  path = "/opt/caddy/data"
  read_only = false
}' | sudo tee -a /etc/nomad.d/nomad.hcl
	fi
fi

# Start Services
if [ "$CLUSTER_TIER" != "lite" ]; then
	cat <<EOF >/etc/systemd/system/consul.service
[Unit]
Description=Consul Agent
Requires=network-online.target
After=network-online.target
[Service]
Restart=on-failure
ExecStart=/usr/local/bin/consul agent -config-dir=/etc/consul.d
[Install]
WantedBy=multi-user.target
EOF
fi

cat <<EOF >/etc/systemd/system/nomad.service
[Unit]
Description=Nomad Agent
Requires=network-online.target
After=network-online.target
[Service]
Restart=on-failure
ExecStart=/usr/local/bin/nomad agent -config=/etc/nomad.d
[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload

if [ "$CLUSTER_TIER" != "lite" ]; then
	systemctl enable consul nomad
	systemctl restart consul nomad
else
	systemctl enable nomad
	systemctl restart nomad
fi
