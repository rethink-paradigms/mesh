#!/bin/bash
#
# Restore Pulumi State from S3 Backup
# Restores Pulumi stack state from S3 backup
#
# Usage: ./restore-pulumi-state.sh <stack_name> [bucket_name] [backup_file]
#
# Arguments:
#   stack_name   - Name of the stack to restore (required)
#   bucket_name  - S3 bucket name (default: my-cluster-backups)
#   backup_file  - Specific backup file to restore (default: pulumi-latest.json)
#

set -e

# Parse arguments
STACK_NAME="${1:-}"
BUCKET_NAME="${2:-my-cluster-backups}"
BACKUP_FILE="${3:-pulumi-latest.json}"

# Validate inputs
if [ -z "$STACK_NAME" ]; then
    echo "Error: Stack name not specified"
    echo "Usage: $0 <stack_name> [bucket_name] [backup_file]"
    echo ""
    echo "Arguments:"
    echo "  stack_name   - Name of the stack to restore (required)"
    echo "  bucket_name  - S3 bucket name (default: my-cluster-backups)"
    echo "  backup_file  - Specific backup file (default: pulumi-latest.json)"
    echo ""
    echo "Examples:"
    echo "  $0 dev"
    echo "  $0 dev my-backups pulumi-dev-2025-01-03-030000.json"
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
S3_PATH="s3://${BUCKET_NAME}/pulumi/${STACK_NAME}/${BACKUP_FILE}"
LOCAL_FILE="/tmp/pulumi-restore-${STACK_NAME}-$(date +%Y%m%d-%H%M%S).json"
LOG_PREFIX="[$(date +%Y-%m-%d\ %H:%M:%S)]"

# Warning prompt
echo ""
echo "╔════════════════════════════════════════════════════════════╗"
echo "║         ⚠️  PULUMI STATE RESTORE - DANGER ZONE  ⚠️          ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""
echo "This operation will OVERWRITE the current Pulumi state for stack: ${STACK_NAME}"
echo ""
echo "Restore source: ${S3_PATH}"
echo "Local file:     ${LOCAL_FILE}"
echo ""
echo "⚠️  WARNING: This operation is IRREVERSIBLE!"
echo "   Any uncommitted changes will be LOST."
echo ""

read -p "Do you want to continue? (yes/no): " -r
echo
if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
    echo "Restore cancelled"
    exit 0
fi

# Step 1: Backup current state
echo "$LOG_PREFIX Step 1: Backing up current state"
CURRENT_BACKUP="/tmp/pulumi-before-restore-${STACK_NAME}-$(date +%Y%m%d-%H%M%S).json"
if pulumi stack select "$STACK_NAME" 2>/dev/null; then
    if pulumi stack export > "$CURRENT_BACKUP" 2>/dev/null; then
        echo "$LOG_PREFIX Current state backed up to: $CURRENT_BACKUP"
    else
        echo "$LOG_PREFIX Warning: Could not export current state (stack may not exist)"
        rm -f "$CURRENT_BACKUP"
    fi
else
    echo "$LOG_PREFIX Stack does not exist, skipping current state backup"
    CURRENT_BACKUP=""
fi

# Step 2: Download backup from S3
echo "$LOG_PREFIX Step 2: Downloading backup from S3"
echo "$LOG_PREFIX S3 path: ${S3_PATH}"

if ! aws s3 cp "$S3_PATH" "$LOCAL_FILE"; then
    echo "Error: Failed to download backup from S3"
    echo "S3 path: ${S3_PATH}"
    exit 1
fi

echo "$LOG_PREFIX Downloaded to: $LOCAL_FILE"

# Step 3: Validate JSON
echo "$LOG_PREFIX Step 3: Validating JSON"
if ! jq empty "$LOCAL_FILE" &> /dev/null; then
    echo "Error: Invalid JSON in backup file"
    rm -f "$LOCAL_FILE"
    exit 1
fi

echo "$LOG_PREFIX JSON validation passed"

# Step 4: Display backup info
echo "$LOG_PREFIX Step 4: Backup information"
echo ""
echo "Backup file:"
jq -r '
  "  Stack: " + .deployment.stack_name +
  "\n  Project: " + .deployment.project_name +
  "\n  Resources: " + (.deployment.resources | length | tostring) +
  "\n  Backend: " + .deployment.backend
' "$LOCAL_FILE"
echo ""

# Step 5: Confirm restore
read -p "Proceed with restore? (yes/no): " -r
echo
if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
    echo "Restore cancelled"
    echo "Cleanup: Removing downloaded file"
    rm -f "$LOCAL_FILE"
    if [ -n "$CURRENT_BACKUP" ]; then
        echo "Previous state backup preserved at: $CURRENT_BACKUP"
    fi
    exit 0
fi

# Step 6: Import state
echo "$LOG_PREFIX Step 5: Importing state to stack: $STACK_NAME"

# Ensure stack exists
if ! pulumi stack select "$STACK_NAME" 2>/dev/null; then
    echo "$LOG_PREFIX Stack does not exist, creating..."
    if ! pulumi stack init "$STACK_NAME"; then
        echo "Error: Failed to create stack"
        exit 1
    fi
fi

# Import state
if ! pulumi stack import --file "$LOCAL_FILE"; then
    echo "Error: Failed to import state"
    echo ""
    echo "Recovery options:"
    if [ -n "$CURRENT_BACKUP" ]; then
        echo "1. Restore previous state: pulumi stack import --file $CURRENT_BACKUP"
    fi
    echo "2. Download backup again: aws s3 cp $S3_PATH /tmp/restore.json"
    exit 1
fi

echo "$LOG_PREFIX State imported successfully"

# Step 7: Verify state
echo "$LOG_PREFIX Step 6: Verifying state"
echo ""
echo "Stack outputs:"
pulumi stack output || true
echo ""

# Step 8: Cleanup
echo "$LOG_PREFIX Cleanup"
echo "Downloaded backup preserved at: $LOCAL_FILE"
if [ -n "$CURRENT_BACKUP" ]; then
    echo "Previous state backup at: $CURRENT_BACKUP"
fi

# Success summary
echo ""
echo "╔════════════════════════════════════════════════════════════╗"
echo "║              ✅ RESTORE COMPLETED SUCCESSFULLY              ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""
echo "Stack: $STACK_NAME"
echo "Backup source: $S3_PATH"
echo ""
echo "Next steps:"
echo "  1. Verify stack: pulumi stack output"
echo "  2. Refresh state: pulumi refresh --yes"
echo "  3. Test infrastructure access"
echo ""
echo "Recovery files (preserve for safety):"
echo "  - Restored backup: $LOCAL_FILE"
if [ -n "$CURRENT_BACKUP" ]; then
    echo "  - Previous state:  $CURRENT_BACKUP"
fi
echo ""

exit 0
