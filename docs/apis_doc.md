# API Documentation

## `/api/v1/topology` [GET]

Extracts and sequences temporal branch topology based on chronological creation.

### Request Output

Requires a valid `repo_path` or corresponding internal `repo_id`.

```json
[
  {
    "hash": "string",
    "short_hash": "string",
    "author": "string",
    "message": "string",
    "date": "string",
    "parents": ["string"]
  }
]
```

## `/ws` [Upgrade: websocket]

Binary endpoint relaying raw `Uint8Array` structs matching the Yjs synchronization protocol.

### Implementation Specifics
The websocket requires clients to send standard Ping/Pong deadlines to enforce connection liveness. Payload maximums are isolated to 10MB bounds to shield buffer overflows.
