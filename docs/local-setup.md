# Local Developer Setup Guide

This document strictly defines the granular steps required to boot, compile, and iteratively debug the monorepo bounds operating dynamically across the internal framework instances.

## Prerequisite Toolchain
Before executing the pipeline locally, developers must map natively specific operational binaries into their OS paths:
*   `Podman` (preferred) or `Docker` daemon mapped logically.
*   `Go 1.22+` mapped dynamically enabling testing internal `apps/backend/` states directly.
*   `Node 20+` enabling rapid mapping testing inside `apps/frontend/`.

---

## 1. Infrastructure Initialization

The absolute initial pipeline state enforces standing up the relational database cleanly enabling the API to track internal states cleanly.

### Steps:
1.  Open absolute `Terminal` or `Shell`.
2.  Navigate specifically into the configuration root: `cd infra/`
3.  Execute the orchestrator matrix: 
    ```bash
    docker-compose up postgres -d
    ```
4.  *(Verification)* Ensure PostgreSQL is cleanly bound natively to `localhost:5432`.

---

## 2. Backend Compiling & Setup

The Golang system natively processes complex binary topologies wrapping Git protocols dynamically.

### Steps:
1.  Navigate into the application boundaries: `cd apps/backend/`
2.  Install internal module specifications:
    ```bash
    go mod download
    ```
3.  Inject localized temporary database routing environmental strings:
    ```bash
    export DATABASE_URL="postgres://git_viz:secret_password@localhost:5432/git_interactive_history?sslmode=disable"
    ```
4.  Boot the localized daemon tracking requests at standard `PORT 8080`:
    ```bash
    go run ./src/main.go
    ```
5.  *(Verification)* Navigating locally to `http://localhost:8080/health` natively returns `OK`.

---

## 3. Frontend WebGL Instantiation

The UI layers bind the `y-websocket` connections testing DOM culling configurations logically mapping PixiJS parameters dynamically.

### Steps:
1.  Open an isolated terminal bounds and navigate into the UI scope: `cd apps/frontend/`
2.  Install the package dependency matrix matching `package.json` logic exactly:
    ```bash
    npm install
    ```
3.  Execute Vite hot-reloading configurations seamlessly:
    ```bash
    npm run dev
    ```
4.  *(Verification)* Navigate the browser instances straight to `http://localhost:3000`. The Glassmorphism interface wrapping the PixiJS visualization canvas will render seamlessly.

---

## Developer Debugging Notes

> [!WARNING]
> If testing bare Git clone engines directly inside the Go module without `podman-compose`, ensure the `apps/backend/repos/` volume exists cleanly to shield the operating system from permission boundary restrictions executing dynamic internal file writes.
