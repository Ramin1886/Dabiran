# Build context: the REPOSITORY ROOT (not apps/frontend) so npm workspaces
# can link the @git-viz/* shared packages during the install.
#   podman build -f apps/frontend/Dockerfile .
FROM docker.io/library/node:20-alpine AS builder

WORKDIR /app

# Workspace manifests + lockfile first for layer-cacheable installs.
COPY package.json package-lock.json ./
COPY packages ./packages
COPY apps/frontend ./apps/frontend

# Install the full workspace tree from the root so symlinks resolve.
RUN npm ci

# Build only the frontend workspace (tsc -b + vite build).
RUN npm run build -w frontend

FROM docker.io/library/nginx:alpine
COPY --from=builder /app/apps/frontend/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
