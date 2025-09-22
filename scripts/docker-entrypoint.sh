#!/bin/bash
set -e

echo "========================================="
echo "Tezos Delegation Service - Starting Up"
echo "========================================="

# Color codes for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to print colored messages
print_status() {
    echo -e "${GREEN}[✓]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[!]${NC} $1"
}

print_error() {
    echo -e "${RED}[✗]${NC} $1"
}

# Wait for PostgreSQL to be ready
wait_for_postgres() {
    print_warning "Waiting for PostgreSQL to be ready..."
    until pg_isready -h ${DB_HOST} -p ${DB_PORT} -U ${DB_USER}; do
        echo "PostgreSQL is unavailable - sleeping"
        sleep 2
    done
    print_status "PostgreSQL is ready!"
}

# Run database migrations
run_migrations() {
    print_warning "Running database migrations..."
    if migrate -path /app/migrations -database "${DATABASE_URL}" up; then
        print_status "Migrations completed successfully"
    else
        print_error "Migration failed"
        return 1
    fi
}

# Restore backup if available and database is empty
restore_backup() {
    print_warning "Checking for backup to restore..."
    
    # Check if database has data
    DELEGATION_COUNT=$(psql "${DATABASE_URL}" -t -c "SELECT COUNT(*) FROM delegations" 2>/dev/null || echo "0")
    
    if [ "$DELEGATION_COUNT" = "0" ]; then
        if [ -f /app/backups/latest.sql.gz ]; then
            print_warning "Restoring database from backup..."
            zcat /app/backups/latest.sql.gz | psql "${DATABASE_URL}"
            print_status "Database restored from backup"
        else
            print_warning "No backup found, starting with empty database"
        fi
    else
        print_status "Database already contains $DELEGATION_COUNT delegations, skipping restore"
    fi
}

# Run tests
run_tests() {
    print_warning "Running tests..."
    
    # Run unit tests
    echo "Running unit tests..."
    if go test -v -short -race ./...; then
        print_status "Unit tests passed"
    else
        print_error "Unit tests failed"
        if [ "${SKIP_TEST_FAILURE}" != "true" ]; then
            exit 1
        fi
    fi
    
    # Run integration tests if enabled
    if [ "${RUN_INTEGRATION_TESTS}" = "true" ]; then
        echo "Running integration tests..."
        if go test -v -tags=integration ./internal/...; then
            print_status "Integration tests passed"
        else
            print_error "Integration tests failed"
            if [ "${SKIP_TEST_FAILURE}" != "true" ]; then
                exit 1
            fi
        fi
    fi
    
    print_status "All tests completed"
}

# Main execution flow
main() {
    echo "Environment: ${ENV:-development}"
    echo "Database: ${DB_HOST}:${DB_PORT}/${DB_NAME}"
    echo ""
    
    # Step 1: Wait for database
    wait_for_postgres
    
    # Step 2: Run migrations
    run_migrations
    
    # Step 3: Restore backup if needed
    if [ "${RESTORE_BACKUP}" = "true" ]; then
        restore_backup
    fi
    
    # Step 4: Run tests
    if [ "${RUN_TESTS}" = "true" ]; then
        run_tests
    fi
    
    # Step 5: Start the application
    print_warning "Starting Tezos Delegation Service..."
    echo "========================================="
    echo ""
    
    # Execute the main application
    exec "$@"
}

# Run main function
main "$@"