#!/bin/bash

# Database backup script for Tezos Delegation Service
set -e

# Configuration
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-tezos_delegations}"
DB_USER="${DB_USER:-tezos}"
DB_PASSWORD="${DB_PASSWORD:-tezos}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
BACKUP_FILENAME="tezos_delegations_$(date +%Y%m%d_%H%M%S).sql.gz"

# Create backup directory if it doesn't exist
mkdir -p "$BACKUP_DIR"

echo "Starting database backup..."
echo "Database: $DB_NAME at $DB_HOST:$DB_PORT"
echo "Backup file: $BACKUP_DIR/$BACKUP_FILENAME"

# Set PGPASSWORD for non-interactive authentication
export PGPASSWORD="$DB_PASSWORD"

# Dump database and compress
pg_dump \
    -h "$DB_HOST" \
    -p "$DB_PORT" \
    -U "$DB_USER" \
    -d "$DB_NAME" \
    --no-owner \
    --no-privileges \
    --data-only \
    --table=delegations \
    --table=schema_migrations \
    --column-inserts \
    --on-conflict-do-nothing \
    | gzip > "$BACKUP_DIR/$BACKUP_FILENAME"

# Get file size
FILESIZE=$(du -h "$BACKUP_DIR/$BACKUP_FILENAME" | cut -f1)

echo "Backup completed successfully!"
echo "File: $BACKUP_DIR/$BACKUP_FILENAME"
echo "Size: $FILESIZE"

# Create a symlink to latest backup
cd "$BACKUP_DIR"
ln -sf "$BACKUP_FILENAME" "latest.sql.gz"
echo "Created symlink: latest.sql.gz -> $BACKUP_FILENAME"
cd - > /dev/null

# Count number of delegations in backup
DELEGATION_COUNT=$(zcat "$BACKUP_DIR/$BACKUP_FILENAME" | grep -c "INSERT INTO delegations" || true)
echo "Delegations in backup: $DELEGATION_COUNT"

# Clean old backups (keep last 5)
echo "Cleaning old backups..."
cd "$BACKUP_DIR"
ls -t *.sql.gz 2>/dev/null | tail -n +6 | xargs -r rm -v

echo "Backup operation completed!"