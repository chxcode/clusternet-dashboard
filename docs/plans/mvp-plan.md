# Clusternet Dashboard MVP Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Build a runnable MVP skeleton for Clusternet Dashboard with Go backend, React/Vite frontend, and Helm chart.

**Architecture:** A single container serves both the Go API and compiled frontend assets. The server is mounted under `BASE_PATH`, defaulting to `/clusternet`. The Helm chart creates read-only RBAC and injects the path as configuration.

**Tech Stack:** Go, React, Vite, TypeScript, Vitest, Helm.

---

## Implemented MVP tasks

1. Create Go config and server tests first.
2. Implement base path normalization and API health endpoints.
3. Add static asset serving and SPA fallback under `/clusternet`.
4. Create React/Vite frontend tests and shell UI.
5. Add Helm chart with Deployment, Service, Ingress, ServiceAccount, and read-only RBAC.
6. Add Dockerfile and README.

## Current verification commands

```bash
go test ./...
cd web && npm test
cd web && npm run build
helm lint charts/clusternet-dashboard
helm template clusternet-dashboard charts/clusternet-dashboard --set ingress.enabled=true --set ingress.ingressClassName=nginx --set ingress.path=/clusternet
```
