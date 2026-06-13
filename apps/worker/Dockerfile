# Multi-stage build for the git-dep-worker CLI.
# Fully-qualified base images so rootless Podman (no unqualified-search
# registries) resolves them without extra host configuration.

# ---- build stage ----
FROM docker.io/library/rust:1-slim AS build
WORKDIR /app

# Cache dependency compilation: copy manifests, build a stub, then the source.
COPY Cargo.toml ./
RUN mkdir -p src \
    && echo 'fn main() {}' > src/main.rs \
    && echo '' > src/lib.rs \
    && cargo build --release || true
COPY src ./src
# Touch sources so cargo rebuilds them after the stub layer.
RUN touch src/main.rs src/lib.rs && cargo build --release

# ---- runtime stage ----
FROM docker.io/library/debian:stable-slim AS runtime
# ureq uses rustls (no system OpenSSL), but CA certs are still needed for TLS
# to public registries when posting over https.
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /app/target/release/git-dep-worker /usr/local/bin/git-dep-worker
ENTRYPOINT ["git-dep-worker"]
CMD ["--help"]
