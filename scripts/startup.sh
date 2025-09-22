#!/bin/bash

# Startup script with automatic restore from backup
set -e

# Configuration
DB_HOST="${DATABASE_URL:-postgres}"
DB_NAME="tezos_delegations"
DB_USER="tezos"
DB_PASSWORD="tezos"
BACKUP_DIR="/backups"
AUTO_RESTORE="${AUTO_RESTORE:-true}"

# Extract host from DATABASE_URL if provided
if [ ! -z "$DATABASE_URL" ]; then
    # Parse PostgreSQL URL: postgres://user:pass@host:port/database
    DB_HOST=$(echo "$DATABASE_URL" | sed -n 's/.*@\([^:]*\):.*/\1/p')
fi

export PGPASSWORD="$DB_PASSWORD"

echo "Tezos Delegation Service - Startup Script"
echo "========================================="

# Wait for database to be ready
echo "Waiting for PostgreSQL to be ready..."
for i in {1..30}; do
    if pg_isready -h "$DB_HOST" -p 5432 -U "$DB_USER" > /dev/null 2>&1; then
        echo "PostgreSQL is ready!"
        break
    fi
    echo "Waiting... ($i/30)"
    sleep 1
done

# Check if database is empty
DELEGATION_COUNT=$(psql -h "$DB_HOST" -p 5432 -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM delegations" 2>/dev/null || echo "0")
DELEGATION_COUNT=$(echo "$DELEGATION_COUNT" | xargs) # Trim whitespace

echo "Current database state: $DELEGATION_COUNT delegations"

# If database is empty and auto-restore is enabled, check for backup
if [ "$DELEGATION_COUNT" = "0" ] && [ "$AUTO_RESTORE" = "true" ]; then
    echo "Database is empty. Checking for backup to restore..."
    
    # List all available backups
    echo "Available backups in $BACKUP_DIR:"
    ls -lha "$BACKUP_DIR"/*.sql.gz 2>/dev/null || echo "  No backup files found"
    
    LATEST_BACKUP="$BACKUP_DIR/latest.sql.gz"
    
    # If latest.sql.gz doesn't exist, try to find any backup
    if [ ! -f "$LATEST_BACKUP" ]; then
        echo "latest.sql.gz not found, looking for any backup file..."
        FIRST_BACKUP=$(ls "$BACKUP_DIR"/*.sql.gz 2>/dev/null | grep -v "latest.sql.gz" | head -n 1)
        if [ ! -z "$FIRST_BACKUP" ]; then
            echo "Found backup: $FIRST_BACKUP"
            cd "$BACKUP_DIR"
            ln -sf "$(basename "$FIRST_BACKUP")" "latest.sql.gz"
            cd - > /dev/null
            echo "Created symlink: $LATEST_BACKUP -> $(basename "$FIRST_BACKUP")"
        fi
    fi
    
    if [ -f "$LATEST_BACKUP" ]; then
        echo "Using backup: $LATEST_BACKUP"
        
        # Get backup info
        BACKUP_SIZE=$(du -h "$LATEST_BACKUP" | cut -f1)
        echo "Backup size: $BACKUP_SIZE"
        
        echo "Restoring from backup..."
        zcat "$LATEST_BACKUP" | psql \
            -h "$DB_HOST" \
            -p 5432 \
            -U "$DB_USER" \
            -d "$DB_NAME" \
            --single-transaction \
            --set ON_ERROR_STOP=on
        
        if [ $? -eq 0 ]; then
            # Verify restoration
            RESTORED_COUNT=$(psql -h "$DB_HOST" -p 5432 -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM delegations" 2>/dev/null || echo "0")
            RESTORED_COUNT=$(echo "$RESTORED_COUNT" | xargs)
            
            if [ "$RESTORED_COUNT" != "0" ]; then
                LAST_TIMESTAMP=$(psql -h "$DB_HOST" -p 5432 -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT MAX(timestamp) FROM delegations")
                LAST_TIMESTAMP=$(echo "$LAST_TIMESTAMP" | xargs)
                
                echo "✓ Successfully restored $RESTORED_COUNT delegations from backup"
                echo "✓ Last indexed timestamp: $LAST_TIMESTAMP"
                echo "✓ Service will continue indexing from this point"
            else
                echo "⚠ Restoration completed but no data found in backup"
            fi
        else
            echo "⚠ Failed to restore from backup, will perform full sync"
        fi
    else
        echo "No backup found at $LATEST_BACKUP"
        echo "Will perform full sync from TzKT API..."
    fi
else
    if [ "$DELEGATION_COUNT" != "0" ]; then
        LAST_LEVEL=$(psql -h "$DB_HOST" -p 5432 -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT MAX(level) FROM delegations")
        LAST_LEVEL=$(echo "$LAST_LEVEL" | xargs)
        echo "Database contains data. Will continue from level $LAST_LEVEL"
    fi
fi

echo ""
echo "Starting Tezos Delegation Service..."
echo "========================================="

# Start the main application
exec /app/tezos-delegation-service