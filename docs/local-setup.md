# Local Setup Guide

1.  Navigate into the `infra/` boundary folder.
2.  Boot the localized mock parameters via:
    `podman-compose -f docker-compose.yml up --build -d`
3.  Access the React Frontend WebGL map at `http://localhost:3000`.
4.  View basic Backend health responses mapped at `http://localhost:8080/health`.

### Environment Boundaries
Ensure `DATABASE_URL` is set to point toward the scoped container database binding within the infra isolated network.
