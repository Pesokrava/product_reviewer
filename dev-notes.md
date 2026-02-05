# Development Notes

## CORS: Not Implemented

**Why**: In production, CORS is handled at the infrastructure layer (API Gateway, Ingress Controller, Service Mesh), not in application code.

**Benefits**:
- Eliminates preflight OPTIONS requests (better performance)
- Separation of concerns - application focuses on business logic
- Single CORS configuration point in Kubernetes/cloud infrastructure
- CORS is browser-only - our consumers are backend services, mobile apps, and CLIs

**Production setup**: `[Browser] → [LB + TLS] → [Gateway + CORS] → [K8s Service] → [API]`

CORS configured at gateway level, not here.

---

## Database Migrations: Run as Kubernetes Jobs

**Why**: Migrations should NOT be run by application pods on startup.

**Problems with app-level migrations**:
- Race conditions when multiple pods start simultaneously
- Failed migrations block entire deployment
- No audit trail or approval process
- Cannot rollback independently from application
- Clutters application startup logic

**Production approach**: Run migrations as Kubernetes Jobs

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: product-reviews-migration-v1-2-3
spec:
  template:
    spec:
      containers:
      - name: migrate
        image: migrate/migrate:v4.15.2
        args:
          - -path=/migrations
          - -database=postgres://...
          - up
        volumeMounts:
        - name: migrations
          mountPath: /migrations
      restartPolicy: Never
  backoffLimit: 3
```

**Benefits**:
- Explicit control over when migrations run
- No race conditions (single job execution)
- Application pods start immediately without waiting for migrations
- Failed migrations don't block app deployment
- GitOps-friendly (ArgoCD/Flux can manage migration jobs)
- Audit trail via Kubernetes Job history

**Development**: Use `make migrate-up` (docker-compose exec) for local development.

---

*Last updated: 2026-02-04*
