# Reflow vs. Docket Comparison

## Overview

| Feature | Grailbio/Reflow | Docket |
| :--- | :--- | :--- |
| **Core Philosophy** | **Functional DSL for Bio/Data**: A bespoke language for defining data pipelines, heavily focused on implicit caching and memoization of file artifacts. | **Code-First Graph Execution**: Define graphs using standard Go code and Protobufs. Focus on type-safe data passing and workflow orchestration. |
| **Definition** | Custom `.reflow` DSL (syntax similar to Go/Rust). | Pure Go structs and function calls. |
| **Data Model** | Files, S3 Objects, and basic primitive types. | **Protobuf Messages**: Strongly typed, schema-driven data exchange. |
| **Execution Model** | **Implicitly Distributed**: Automatically provisions cloud instances (EC2) to run Docker containers. | **Embeddable/Host-Agnostic**: Runs within your existing Go application or worker. Can be distributed (via Temporal/River) but doesn't enforce a specific scheduler. |
| **Memoization** | **Centralized CAS**: Uses a global Content-Addressable Storage (S3/DynamoDB) to cache results forever based on input hashes. | **Ephemaral/Scoped**: Caches results within a single execution. Durable persistence is an opt-in feature (via River/Postgres). |

## Key Learnings from Reflow for Docket

Reflow was built for "big data" pipelines where steps (Docker containers) take hours and cost real money. Docket is built for "business logic" graphs where steps might be API calls or fast computations. However, Reflow has solved some hard problems that Docket will eventually face.

### 1. The "Files" Problem (Data Blob Management)
*   **Reflow:** Treats files as first-class citizens. A file is identified by the SHA256 hash of its content, not its path. This enables perfect caching.
*   **Docket Today:** Passes Protobuf messages. If a step produces a 1GB CSV, you probably don't want that in a Proto.
*   **Lesson:** Docket needs a standard way to handle **Blob References**.
    *   *Action:* Create a standard `proto.Blob` message type that wraps a URI (s3://...) and a digest (sha256:...).
    *   *Feature:* Steps can return these Blobs, and the framework can optionally verify hashes or handle download/upload transparently.

### 2. Aggressive Memoization (Caching)
*   **Reflow:** If you run the same pipeline twice with the same inputs, the second run finishes instantly because every step's output is cached globally by input hash.
*   **Docket Today:** Re-executes logic every time unless manually skipped.
*   **Lesson:** **Deterministic Caching** is powerful.
    *   *Action:* Implement a `CalculateCacheKey(step, inputs)` function.
    *   *Feature:* If a step is marked `Pure` or `Cacheable`, check a persistent store (Redis/Postgres) for a result before executing. This turns Docket into a build system (like Bazel) for business logic.

### 3. Resource Requirements
*   **Reflow:** Allows specifying `cpu`, `mem`, `disk` for every step. The scheduler finds a machine that fits.
*   **Docket Today:** Assumes all steps run happily in the current process.
*   **Lesson:** As Docket scales to "Async Workers" (via River), we need to route work.
    *   *Action:* Add `WithResources(cpu=2, ram="4GB")` to `Register()`.
    *   *Feature:* When offloading to River/Temporal, map these requirements to specific Task Queues or Worker Pools optimized for those workloads.

### 4. Lazy Evaluation (Internals)
*   **Reflow:** The DSL is lazy; nothing happens until you ask for a specific output value.
*   **Docket Today:** `Execute` runs the whole graph to produce the target.
*   **Lesson:** Docket is already essentially lazy (it only resolves dependencies of the requested target), but we could make this explicit.
    *   *Feature:* "Partial Execution". Allow a user to ask for an intermediate node's result, and Docket only runs the subgraph necessary to produce that specific intermediate value.

## Roadmap Additions

Based on this comparison, here are high-value additions to the Docket roadmap:

1.  **P2: Blob/Asset Support:** Standardize how large files are passed between steps (References vs. Value).
2.  **P2: Global Cache Layer:** Middleware to check a KV store for cached step results based on input hash.
3.  **P3: Resource Hints:** Metadata for steps to hint to the executor (or external scheduler) about concurrency/memory needs.
