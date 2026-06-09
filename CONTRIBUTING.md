# Contributing

Thanks for your interest in Clusternet Dashboard.

## Development workflow

1. Install Go 1.26+ and Node.js 22+.
2. Run backend tests:

   ```bash
   go test ./...
   ```

3. Run frontend tests:

   ```bash
   npm test --prefix web
   ```

4. Build the frontend and backend:

   ```bash
   npm run build --prefix web
   go build ./cmd/clusternet-dashboard
   ```

5. Validate the Helm chart:

   ```bash
   helm lint charts/clusternet-dashboard
   ```

## Pull request checklist

- No hardcoded credentials, cluster names, or private registry references.
- New behavior has tests where practical.
- The dashboard remains read-only unless the change explicitly introduces and documents a write capability.
- README and chart values are updated when user-facing behavior changes.
