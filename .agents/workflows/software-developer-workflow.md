---
description: 
---

# Operational Workflow

## Phase 1: Architecture & Proposal (Wait for Approval)

* **Analyze:** Carefully review the system requirements, performance bottlenecks, or specific issue provided.
* **Propose:** Detail a structured architectural design, system interaction model, or algorithmic approach before writing any code.
* **Compare:** If multiple viable approaches or technology choices exist (e.g., Go vs. Rust for a specific microservice, gRPC vs. REST), provide an objective, technical comparison focusing on memory safety, concurrency, and network overhead.
* **Evaluate:** Explicitly list the computational complexity, advantages, disadvantages, operational overhead, and potential edge-case risks (e.g., race conditions, network partitions) of your proposed method(s).
* **Halt:** Stop generation and explicitly ask for my "green light" or approval. Do not generate any implementation code, Dockerfiles, or Kubernetes manifests during Phase 1.

## Phase 2: Implementation (Post-Approval)

Upon receiving my explicit approval, adhere strictly to these coding standards:

* **Complete Code:** Provide the complete, functional code blocks. Do not use partial snippets, placeholders (e.g., `// ... rest of code`), or abbreviations.
* **Strict Scope:** Modify or add only the code directly required to resolve the specific issue or implement the agreed-upon architecture. Leave all unrelated logic exactly as it was provided.
* **Comment Preservation:** Retain all existing comments verbatim. Only alter or remove comments if the underlying logic has changed, making the old comment factually incorrect.
* **Modularity:** It is highly important to implement in a modular manner. Do not write single scripts with 1000+ lines of code. Separate the code into as many files as required.
* **Function Comments:** All functions must have clear comments explaining their exact purpose, inputs, and outputs.

## Phase 3: System Documentation

Generate extensive documentation alongside your code, strictly following these requirements:

* **Format:** Provide the system and API documentation strictly in `.md` format only.
* **Native Visuals:** Incorporate tables and native architectural graphs. Use Mermaid.js syntax for sequence, state, or infrastructure diagrams.
* **Strict Constraint:** NEVER include versioning information, revision histories, or timestamps anywhere in the documentation.
* **API Documentation:** All APIs must have detailed documentation.

## Phase 4: Repository Structure & Organization

The repository must be organized in the following consistent pattern:

```text
project-root/
├── cicd/                           # Continuous Integration and Deployment workflows
│   ├── build.yaml                  # Build steps for web apps and packages
│   └── deploy.yaml                 # Deployment steps for infrastructure and apps
├── docs/                           # Internal documentation and guides
│   ├── architecture.md             # High-level system architecture
│   ├── local-setup.md              # Instructions for local development
│   ├── apis_doc.md                 # APIs documentation
│   ├── functions_doc.md            # Functions documentation
│   ├── features_doc.md             # Software feature documentation
│   └── adr/                        # Architecture Decision Records
├── apps/                           # Deployable application services (Web purpose)
│   ├── frontend/                   # Web interface
│   │   ├── src/                    # UI components, pages, and web assets
│   │   ├── package.json            # Frontend-specific dependencies
│   │   └── Dockerfile              # Containerization for the frontend app
│   └── backend/                    # API and backend services 
│       ├── src/                    # API routes, business logic, models
│       ├── package.json            # Backend-specific dependencies
│       └── Dockerfile              # Containerization for the backend app
├── packages/                       # Shared internal libraries (Web purpose)
│   ├── ui-components/              # Shared component library across frontend apps
│   ├── shared-types/               # Shared TypeScript interfaces or data contracts
│   └── utils/                      # Common helper functions and utilities
├── infra/                          # Cloud-native infrastructure (Cloud purpose)
│   ├── terraform/                  # Infrastructure as Code (e.g., AWS, GCP, Azure)
│   │   ├── modules/                # Reusable IaC modules (VPC, Databases)
│   │   └── environments/           # Environment states (dev, staging, prod)
│   ├── kubernetes/                 # Kubernetes manifests and GitOps configurations
│   │   ├── base/                   # Core definitions and Helm charts
│   │   └── overlays/               # Environment-specific overrides (via Kustomize)
│   └── docker-compose.yml          # Local environment orchestration for all services
└── README.md                       # Main entry point linking to the docs/ directory

* **Root README: Each project repository must have a README.md at the root level.
* **Documentation Links: The root README.md must contain links to other related documentations (e.g., linking directly to files within the docs/ directory).