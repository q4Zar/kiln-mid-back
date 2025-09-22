#!/bin/bash

# Database restore script for Tezos Delegation Service
set -e

# Configuration
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-tezos_delegations}"
DB_USER="${DB_USER:-tezos}"
DB_PASSWORD="${DB_PASSWORD:-tezos}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
BACKUP_FILE="${1:-$BACKUP_DIR/latest.sql.gz}"

# Check if backup file exists
if [ ! -f "$BACKUP_FILE" ]; then
    echo "Error: Backup file not found: $BACKUP_FILE"
    echo "Usage: $0 [backup_file.sql.gz]"
    echo "If no file specified, will use: $BACKUP_DIR/latest.sql.gz"
    exit 1
fi

echo "Starting database restore..."
echo "Database: $DB_NAME at $DB_HOST:$DB_PORT"
echo "Backup file: $BACKUP_FILE"

# Set PGPASSWORD for non-interactive authentication
export PGPASSWORD="$DB_PASSWORD"

# Check if database has existing data
EXISTING_COUNT=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM delegations" 2>/dev/null || echo "0")
EXISTING_COUNT=$(echo "$EXISTING_COUNT" | xargs) # Trim whitespace

if [ "$EXISTING_COUNT" != "0" ]; then
    echo "Warning: Database already contains $EXISTING_COUNT delegations"
    read -p "Do you want to clear existing data? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Clearing existing delegations..."
        psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "TRUNCATE TABLE delegations RESTART IDENTITY;"
    else
        echo "Restore cancelled."
        exit 1
    fi
fi

# Get file size
FILESIZE=$(du -h "$BACKUP_FILE" | cut -f1)
echo "Restoring from backup (Size: $FILESIZE)..."

# Restore database
zcat "$BACKUP_FILE" | psql \
    -h "$DB_HOST" \
    -p "$DB_PORT" \
    -U "$DB_USER" \
    -d "$DB_NAME" \
    --single-transaction \
    --set ON_ERROR_STOP=on

# Verify restoration
RESTORED_COUNT=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM delegations")
RESTORED_COUNT=$(echo "$RESTORED_COUNT" | xargs) # Trim whitespace

# Get last indexed level
LAST_LEVEL=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT MAX(level) FROM delegations")
LAST_LEVEL=$(echo "$LAST_LEVEL" | xargs) # Trim whitespace

# Get date range
DATE_RANGE=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT MIN(timestamp)::date || ' to ' || MAX(timestamp)::date FROM delegations")
DATE_RANGE=$(echo "$DATE_RANGE" | xargs) # Trim whitespace

echo ""
echo "Restore completed successfully!"
echo "Delegations restored: $RESTORED_COUNT"
echo "Last indexed level: $LAST_LEVEL"
echo "Date range: $DATE_RANGE"
echo ""
echo "The service will continue indexing from level $LAST_LEVEL on next startup."