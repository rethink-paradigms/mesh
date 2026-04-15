#!/bin/bash
#
# Backup Pulumi State to S3
# Backs up all Pulumi stacks to S3 for disaster recovery
#
# Usage: ./backup-pulumi-state.sh [bucket_name]
#
# Environment Variables:
#   PULUMI_BACKUP_BUCKET - S3 bucket name (overrides argument)
#   PULUMI_STACKS - Space-separated list of stacks to backup
#

set -e

# Default configuration
DEFAULT_BUCKET="${PULUMI_BACKUP_BUCKET:-my-cluster-backups}"
DEFAULT_STACKS="dev"

# Parse arguments
BUCKET_NAME="${1:-$DEFAULT_BUCKET}"
STACKS="${PULUMI_STACKS:-$DEFAULT_STACKS}"

# Validate inputs
if [ -z "$BUCKET_NAME" ]; then
    echo "Error: S3 bucket name not specified"
    echo "Usage: $0 [bucket_name]"
    echo "Environment: PULUMI_BACKUP_BUCKET"
    exit 1
fi

# Check required commands
for cmd in pulumi aws jq date; do
    if ! command -v "$cmd" &> /dev/null; then
        echo "Error: Required command not found: $cmd"
        exit 1
    fi
done

# Configuration
TIMESTAMP=$(date +%Y-%m-%d-%H%M%S)
LOG_PREFIX="[$(date +%Y-%m-%d\ %H:%M:%S)]"

echo "$LOG_PREFIX Starting Pulumi state backup"
echo "$LOG_PREFIX Bucket: $BUCKET_NAME"
echo "$LOG_PREFIX Stacks: $STACKS"

# Function to backup a single stack
backup_stack() {
    local stack="$1"
    local backup_file="/tmp/pulumi-${stack}-${TIMESTAMP}.json"
    local s3_prefix="s3://${BUCKET_NAME}/pulumi/${stack}"

    echo "$LOG_PREFIX Processing stack: $stack"

    # Select stack
    if ! pulumi stack select "$stack" &> /dev/null; then
        echo "$LOG_PREFIX Warning: Stack '$stack' not found, skipping"
        return 1
    fi

    # Export state
    echo "$LOG_PREFIX Exporting state for stack: $stack"
    if ! pulumi stack export > "$backup_file"; then
        echo "$LOG_PREFIX Error: Failed to export state for stack: $stack"
        rm -f "$backup_file"
        return 1
    fi

    # Validate JSON
    echo "$LOG_PREFIX Validating JSON for stack: $stack"
    if ! jq empty "$backup_file" &> /dev/null; then
        echo "$LOG_PREFIX Error: Invalid JSON for stack: $stack"
        rm -f "$backup_file"
        return 1
    fi

    # Get file size
    file_size=$(wc -c < "$backup_file")
    echo "$LOG_PREFIX State file size: ${file_size} bytes"

    # Upload to S3
    echo "$LOG_PREFIX Uploading to S3: ${s3_prefix}/pulumi-${stack}-${TIMESTAMP}.json"
    if ! aws s3 cp "$backup_file" "${s3_prefix}/pulumi-${stack}-${TIMESTAMP}.json"; then
        echo "$LOG_PREFIX Error: Failed to upload to S3 for stack: $stack"
        rm -f "$backup_file"
        return 1
    fi

    # Update latest marker
    echo "$LOG_PREFIX Updating latest marker for stack: $stack"
    if ! aws s3 cp "$backup_file" "${s3_prefix}/pulumi-latest.json"; then
        echo "$LOG_PREFIX Warning: Failed to update latest marker for stack: $stack"
    fi

    # Cleanup local file
    rm -f "$backup_file"

    echo "$LOG_PREFIX Successfully backed up stack: $stack"
    return 0
}

# Backup each stack
success_count=0
failure_count=0

for stack in $STACKS; do
    if backup_stack "$stack"; then
        ((success_count++))
    else
        ((failure_count++))
    fi
done

# Summary
echo "$LOG_PREFIX Backup completed"
echo "$LOG_PREFIX Success: $success_count stacks"
echo "$LOG_PREFIX Failed: $failure_count stacks"

# Exit with error if any backups failed
if [ "$failure_count" -gt 0 ]; then
    exit 1
fi

exit 0
