# Database Backup and Restore

The Tezos Delegation Service includes built-in backup and restore functionality to avoid re-syncing the entire blockchain history on startup.

## Features

- Automatic database restoration on startup if database is empty
- Manual backup and restore commands
- Compressed SQL backups for efficient storage
- Automatic cleanup of old backups (keeps last 5)
- Fast startup from existing data

## Automatic Restore on Startup

When the service starts with an empty database, it will automatically:
1. Check for a backup file at `/backups/latest.sql.gz`
2. If found, restore the database from the backup
3. Continue indexing from the last indexed level
4. If no backup found, perform full sync from TzKT API

To enable/disable automatic restore, set the `AUTO_RESTORE` environment variable:
```yaml
AUTO_RESTORE: "true"  # Enable automatic restore (default)
AUTO_RESTORE: "false" # Disable automatic restore
```

## Manual Backup

Create a backup of the current database state:

```bash
# Using Make command
make backup

# Using Docker Compose directly
docker-compose exec postgres sh -c 'PGPASSWORD=tezos pg_dump -U tezos -d tezos_delegations --data-only' | gzip > backups/backup.sql.gz

# Using backup script in container
docker-compose exec tezos-delegation-service /app/backup.sh
```

Backups are stored in the `./backups` directory with timestamps.

## Manual Restore

Restore database from a backup:

```bash
# Restore from latest backup
make restore

# Restore from specific file
zcat backups/tezos_delegations_20240101_120000.sql.gz | docker-compose exec -T postgres psql -U tezos -d tezos_delegations

# Using restore script in container
docker-compose exec tezos-delegation-service /app/restore.sh /backups/latest.sql.gz
```

## Check Database Status

View current database state:

```bash
# Using Make command
make db-status

# This will show:
# - Total number of delegations
# - Last indexed block level
# - Date range of stored data
```

## Backup File Format

Backups are created as:
- Compressed SQL files (`.sql.gz`)
- Include only data (no schema)
- Use column-inserts format for compatibility
- Include ON CONFLICT DO NOTHING for safe restoration
- Named with timestamp: `tezos_delegations_YYYYMMDD_HHMMSS.sql.gz`
- Symlinked to `latest.sql.gz` for easy access

## Performance

- Backup: ~1-2 minutes for 500,000 delegations
- Restore: ~30-60 seconds for 500,000 delegations
- File size: ~50-100MB compressed for 500,000 delegations
- Startup with restore: <1 minute vs 30+ minutes for full sync

## Docker Volumes

The backup directory is mounted as a volume:
```yaml
volumes:
  - ./backups:/backups
```

This ensures backups persist between container restarts.

## Best Practices

1. **Regular Backups**: Create backups periodically, especially after major sync operations
2. **Before Updates**: Always backup before updating the service
3. **Storage Management**: Old backups are automatically cleaned (keeps last 5)
4. **Verify Backups**: Use `make db-status` to verify backup restoration
5. **Production Use**: Store backups in external storage (S3, NFS, etc.) for production

## Troubleshooting

### Backup fails with "connection refused"
- Ensure PostgreSQL container is running: `docker-compose up -d postgres`
- Wait for health check: `docker-compose ps`

### Restore fails with "database does not exist"
- Run migrations first: `docker-compose up -d postgres`
- Wait for database initialization

### Automatic restore not working
- Check backup file exists: `ls -la backups/latest.sql.gz`
- Verify AUTO_RESTORE is set to "true"
- Check container logs: `docker-compose logs tezos-delegation-service`

### Restore takes too long
- Ensure sufficient disk I/O performance
- Consider using SSD storage
- Monitor with: `docker stats`