# Collaborative Git Visualization Platform

Welcome to the Collaborative Git Visualization Platform repository. This platform renders large-scale branch topologies interactively via WebGL and syncs real-time presence/CRDT notes securely over WebSockets.

## Documentation Navigation

This monorepo adheres strictly to enterprise standards. Please refer to the relevant documentation sections below:

*   [Overall Architecture](./docs/architecture.md)
*   [Local Setup Guide](./docs/local-setup.md)
*   [API Documentation](./docs/apis_doc.md)
*   [Functions Documentation](./docs/functions_doc.md)
*   [Features Documentation](./docs/features_doc.md)

## Monorepo Layout

*   `apps/frontend`: WebGL rendering layer (React + PixiJS).
*   `apps/backend`: Bare-git daemon wrapper and WebSocket Yjs relay (Golang).
*   `infra/`: Infrastructure deployments and isolated PostgreSQL mapping.
*   `docs/`: Extensive project documentation and architecture decision records (ADRs).
