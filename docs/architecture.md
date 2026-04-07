# Architecture Overview

This document describes the high-level system interactions of the Collaborative Git Visualization Platform.

## Core Topology

The system uses an asynchronous stateless UI binding via WebGL (`frontend/`), communicating with an interactive binary websocket relay and pure-git integration core (`backend/`). Storage maintains strict relational rules in PostgreSQL.

## Flow Diagram

```mermaid
graph TD
    UI[Frontend: React + PixiJS WebGL]
    UI_CRDT[Frontend: Yjs Memory Document]
    
    API[Backend: API Router]
    GIT[Backend: Git Engine Wrapper]
    WS[Backend: Websocket Hub Relay]
    DB[(PostgreSQL Database)]

    UI -- REST (Fetch Topology) --> API
    API -- HTTPS/SSH Transport --> GIT
    
    UI_CRDT -- CRDT Binary Delta Stream --> WS
    WS -- Broadcast State --> UI_CRDT
    
    WS -- Async Snapshot --> DB
    API -- Read Tenant Models --> DB
```

## Security Posture
*   Credentials encrypted symmetrically utilizing AES-256-GCM.
*   Containers executed entirely rootless.
