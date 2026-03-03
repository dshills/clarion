# Runbook

## Starting the Service

1. Set DATABASE_URL to the postgres-datastore connection string.
2. Start the api-server with the standard entrypoint.

## Health Checks

- GET /users should return 200 OK when the service is healthy.
