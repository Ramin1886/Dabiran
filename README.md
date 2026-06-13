# Collaborative Git Visualization Platform

## Project Purpose & Description Software
The Collaborative Git Visualization Platform is an elite, cloud-native application designed to radically transform how development teams interact with Git repository histories. By abstracting the standard CLI log into an interactive, real-time collaborative WebGL canvas, it resolves the difficulty of mapping massive branch topologies, evaluating structural splits/merges, and managing historical commit knowledge. 

The software strictly targets self-hosted customer environments enforcing zero-trust data sovereignty dynamically mapping `HTTPS` and `SSH` credentials to localized bare bare-repositories. The software provides an infinite, Miro-style collaborative workspace where architects can annotate, visually link cross-dependencies, and traverse chronological topologies effortlessly across thousands of data points asynchronously synchronized via zero-conflict `CRDT` WebSockets.

## Documentation Navigation

This monorepo adheres strictly to enterprise operational workflow parameters.

*   [Overall Architecture](./docs/architecture.md)
*   [Technology Stack Matrix](./docs/tech_stack.md)
*   [Developer Local Setup Guide](./docs/local-setup.md)
*   [Features Matrix](./docs/features_doc.md)
*   [API Schema Definitions](./docs/apis_doc.md)
*   [Internal Function Parameters](./docs/functions_doc.md)
*   [Feature Development TODO Map](./docs/todo_features.md)

## Monorepo Layout Bounds

*   `apps/frontend`: WebGL rendering layer leveraging React and PixiJS.
*   `apps/backend`: Bare-git daemon wrapper and WebSocket Yjs relay written in Golang.
*   `infra/`: Infrastructure deployments encompassing Podman contexts and PostgreSQL bindings.
*   `docs/`: Extensive internal blueprinting, step-by-step configurations, and architectural guides.
