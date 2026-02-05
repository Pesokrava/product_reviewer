# Development Notes

## CORS: Not Implemented

**Why**: In production, CORS is handled at the infrastructure layer (API Gateway, Ingress Controller, Service Mesh), not in application code.

**Benefits**:
- Eliminates preflight OPTIONS requests (better performance)
- Separation of concerns - application focuses on business logic
- Single CORS configuration point in Kubernetes/cloud infrastructure

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
          - -database=postgres://...\
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
- Failed migrations don't block app deployment
- GitOps-friendly (ArgoCD/Flux can manage migration jobs)
- Audit trail via Kubernetes Job history

**Startup probe**: Application pods use a startup probe that verifies the latest migrations are applied before accepting traffic. If migrations are pending, the pod fails the probe and won't start. This ensures zero-downtime deployments - migration Job completes first, then pods start and pass their probes.

**Development**: Use `make migrate-up` (docker-compose exec) for local development. Schema drift is managed manually by developers in this environment.

**Production**: A dedicated Kubernetes Job for migrations is triggered as part of the CI/CD pipeline when a new version of the application is deployed, ensuring migrations are applied before application pods start.

---

## Rating Calculation: No Dead Letter Queue (DLQ) for Events

While a simpler approach using a database trigger could have handled rating recalculations, the use of a message broker (NATS JetStream) was chosen to meet a requirement for an event-driven architecture, even if it introduced more complexity than a direct database solution.

**Why no DLQ**:
- Rating calculation prioritizes eventual consistency over immediate, strict consistency. Minor inconsistencies are tolerable given the self-correcting nature of the process.
- The rating calculation is idempotent and performed directly on the database state. Each new review event for a product effectively triggers a full recalculation, which corrects any missed updates from previous failed attempts.
- Events are discarded after 3 failed processing attempts by the rating worker. This design choice simplifies the event processing flow, avoiding the operational overhead of managing a DLQ for non-critical, self-correcting updates.

**Implications for Auditability/Debugging**:
- Persistently failing rating updates would manifest as prolonged periods of slightly inaccurate product ratings rather than accumulating in a DLQ.
- Monitoring worker logs for errors and implementing alerts for significant or sustained rating discrepancies would be crucial for identifying and debugging underlying issues.

---

## Caching Strategy: Aggressive Invalidation

Aggressive cache invalidation ensures timely consistency for user-facing data. By invalidating both product rating and review list caches after a write, it makes new reviews quickly visible and ensures the cache reflects the most up-to-date average rating from the database, balancing consistency with operational simplicity.

---

## Database Choices: PostgreSQL & SQLX

**PostgreSQL**: Selected for its proven robustness, reliability, and strong support for SQL standards and advanced features like JSONB. Its open-source nature and widespread adoption also contribute to its popularity and ease of use.

**SQLX (Raw SQL with Helpers)**: Chosen over a full Object-Relational Mapper (ORM) to maintain direct control over SQL queries for performance and clarity. SQLX, which extends Go's standard `database/sql`, offers a good balance by simplifying common tasks like scanning query results into structs while still encouraging explicit SQL writing. This approach avoids ORM complexities and impedance mismatch, fitting well with Go's idiomatic style.

---

## Development Tooling

During the development and documentation process, both Claude Code (claude.ai/code) and Gemini CLI were utilized as AI assistants.

