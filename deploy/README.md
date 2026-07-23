# Deployment

The M2 local stack uses exact PostgreSQL and Redis image versions. Local database
backups and Redis persistence are intentionally disabled; production persistence
is designed and verified in M6.

From the repository root:

```bash
docker compose -f deploy/compose.yaml up --build
docker compose -f deploy/compose.yaml down
```

Use `down --volumes` only when intentionally resetting all local data.
