#!/bin/bash
set -e

APP_NAME="marketing-site"
IMAGE_NAME="marketing-site"
TAG="local"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$(dirname "$DIR")")"
OPS_DIR="$ROOT_DIR/ops-platform"

echo "🚀 Simulating CI/CD for $APP_NAME..."

# 1. Build Docker Image (Local Host)
echo "📦 Building Docker Image..."
# We use the host's docker daemon
docker build -t $IMAGE_NAME:$TAG $DIR

# 2. Transfer Image to Multipass (Local Dev Optimization)
echo "🚚 Transferring image to Cluster Nodes..."

# Save image
echo "   saving image to tar..."
docker save $IMAGE_NAME:$TAG > /tmp/$APP_NAME.tar

# Load into Leader
echo "   copying to local-leader..."
multipass transfer /tmp/$APP_NAME.tar local-leader:/tmp/$APP_NAME.tar

echo "   importing into local-leader docker..."
multipass exec local-leader -- sudo docker load -i /tmp/$APP_NAME.tar

# Cleanup
rm /tmp/$APP_NAME.tar
multipass exec local-leader -- rm /tmp/$APP_NAME.tar

echo "✅ Image ready on Cluster."

# 3. Sync Secrets
echo "🔐 Syncing Secrets..."
# Create a dummy .env for the app
echo "DB_PASSWORD=secret-123" > $DIR/.env.local
python3 $OPS_DIR/scripts/sync_secrets.py \
  --source file \
  --env-file $DIR/.env.local \
  --job "jobs/$APP_NAME/secrets" \
  --address "http://$(multipass info local-leader --format json | jq -r .info.\"local-leader\".ipv4[0]):4646"

# 4. Render Template
echo "📄 Rendering Template..."
TEMPLATE="$OPS_DIR/templates/jobs/core/web-service.nomad.hcl"
sed "s/JOB_NAME_PLACEHOLDER/$APP_NAME/g" $TEMPLATE > $DIR/deployed.nomad

# 5. Deploy
echo "🚀 Deploying to Nomad..."
# We need to target the leader
NOMAD_ADDR="http://$(multipass info local-leader --format json | jq -r .info.\"local-leader\".ipv4[0]):4646"

nomad job run \
  -address="$NOMAD_ADDR" \
  -var="app_name=$APP_NAME" \
  -var="image=$IMAGE_NAME" \
  -var="image_tag=$TAG" \
  -var="port=80" \
  -var="count=1" \
  $DIR/deployed.nomad

echo "✅ Deployed! Access at http://$APP_NAME.localhost (mapped to Leader IP)"
