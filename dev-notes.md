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

*Last updated: 2026-02-04*
