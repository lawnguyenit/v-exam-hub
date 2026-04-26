# AWS ECS deployment notes

## Database

Do not run PostgreSQL as a normal app container for production. Use Amazon RDS for PostgreSQL.

Reasons:
- ECS tasks are replaceable. A database container inside ECS can lose data if storage, backup, and restore are not designed carefully.
- RDS handles backups, patching, failover options, monitoring, storage growth, and security groups.
- The app already reads the database connection from `DB_URL`, so switching from local Postgres to RDS only needs environment/secrets changes.

Use the Postgres container only for local development or a short-lived demo.

## Required backend environment

For production, set:

```env
APP_ENV=production
PORT=8080
DB_URL=host=<rds-endpoint> port=5432 user=<user> password=<password> dbname=<db> sslmode=require
DB_STARTUP_TIMEOUT=90s
```

Store `DB_URL` in AWS Secrets Manager or SSM Parameter Store, not in the task definition JSON directly.

## Frontend to backend routing

The React app calls `/api/...` on the same origin. The routing decision belongs to nginx or the ALB.

If frontend and backend are in the same ECS task:

```env
BACKEND_UPSTREAM=http://127.0.0.1:8080
```

If frontend and backend are separate ECS services with Cloud Map:

```env
BACKEND_UPSTREAM=http://<backend-service>.<namespace>:8080
```

If the ALB routes `/api/*` directly to the backend target group, nginx proxying is not on the critical path for API traffic.

## Images

Push application images to ECR:

- backend image from root `Dockerfile`
- frontend image from `frontend/Dockerfile`

Do not push a custom PostgreSQL image for production unless there is a very specific operational reason.
