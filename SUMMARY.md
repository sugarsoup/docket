# Protograph: Executive Summary

## What is Protograph?

Protograph is a high-performance, developer-friendly framework for building multi-step data processing pipelines in Go. It's designed as a lightweight alternative to Temporal for teams that need to orchestrate complex, interdependent computations at scale without the operational overhead, performance limitations, or learning curve of traditional workflow engines. At its core, Protograph treats data pipelines as dependency graphs where each step declares what it needs (via Protocol Buffer types) and what it produces, and the system automatically resolves execution order, manages retries, and provides crash recovery through checkpointing.

## Why Protograph Exists

Protograph emerged from real production pain points with Temporal: it couldn't scale beyond a few dozen workflows per second on Postgres, required massive Cassandra clusters to achieve acceptable throughput, imposed too much latency for real-time use cases, and came with significant operational complexity. Beyond performance, Temporal creates a walled garden—you must deeply understand its proprietary execution model, work within its constraints, and manage specialized infrastructure. Teams just wanted reliable multi-step data processing without becoming Temporal experts or operating complex distributed systems. Protograph is what you wish existed when you needed Temporal: something that scales on standard Postgres, runs fast enough for synchronous RPC calls, doesn't require specialized infrastructure knowledge, and lets you focus on your business logic rather than framework internals.

## Core Design Principles

**Optimize for the happy path.** Most computations succeed on the first attempt. Rather than paying the durability cost on every step like Temporal does, Protograph executes entire dependency chains in-memory at full speed and only uses the database as a checkpoint system for crash recovery. This architectural choice delivers 10-100x better throughput while maintaining reliability—you get the speed of in-memory computation with the safety of persistent checkpoints, but only when actually needed.

**Declarative dependencies through types.** Instead of manually wiring steps together and passing giant mutable objects through your pipeline, steps declare dependencies through their function signatures using Protocol Buffer types. The system automatically resolves what needs to run and in what order. If a step needs `UserEvents` and `UserProfile`, it just declares those parameters—the graph figures out how to produce them by recursively walking dependencies. This GraphQL-style resolution eliminates the brittle manual orchestration that makes pipelines hard to change and reason about.

**One output type, one path.** Each Protocol Buffer output type maps to exactly one step that produces it. This constraint makes the dependency graph deterministic and unambiguous—there's never confusion about "which step should produce this?" This simplicity enables powerful automatic features like validation (can all dependencies be resolved?), visualization (show me the execution plan), and optimization (what can run in parallel?) without complex configuration.

**Just Postgres and your application.** Protograph uses PostgreSQL's `SELECT FOR UPDATE SKIP LOCKED` pattern for job distribution, which provides natural horizontal scaling, future scheduling (via timestamp columns), and database-level visibility for operations. You don't need Kafka, Redis, or specialized message brokers—query your job queue directly with SQL for monitoring and management. This radically simplifies operations compared to systems that require additional coordination infrastructure.

**Not a walled garden.** Protograph is just Go code calling Go functions with standard Protocol Buffers. You don't need to learn a proprietary execution model, understand replay semantics, or debug why your workflow is stuck in framework internals. If something breaks, you're looking at your own code running normally with clear stack traces. The learning curve is: "write a function that takes protos and returns a proto"—that's the entire programming model.

## Key Technical Insights

**Checkpoint asynchronously, only on failure.** This is the critical performance insight. Temporal persists after every activity, creating a synchronous database round-trip on the hot path. Protograph persists asynchronously to a background queue and only reads from the database on retry after a crash. The happy path (most executions) touches the database zero times for reads. The checkpoint system exists purely for fault tolerance, not performance, inverting the typical durability-first architecture.

**Dependency injection through function signatures.** The system distinguishes three contexts with different lifecycles: graph-lifetime dependencies (database clients, API clients) injected via constructors at registration time, execution-lifetime context (execution ID, tracing, cancellation) passed as the first parameter to every step, and proto dependencies (the actual data) passed as subsequent parameters and resolved automatically. This separation keeps concerns clean—infrastructure setup happens once at graph initialization, execution tracking flows through standard Go context patterns, and business logic focuses purely on data transformations.

**Content-addressed checkpointing with workflow scoping.** Checkpoints are keyed by `(workflow_id, output_type, input_hash)` creating natural isolation between execution attempts while enabling efficient lookup. Within a single execution, intermediate outputs live in memory; only if a step fails and the execution retries does the system check for previously computed checkpoints. Once an execution completes or is abandoned, its checkpoints become garbage and can be cleaned up aggressively (1 hour TTL), keeping the checkpoint table small and fast.

**Validation at graph construction time.** Before any executions run, the graph validates that all dependencies can be resolved, there are no cycles, required runtime dependencies (API clients, databases) are provided, and the one-step-per-output-type constraint is satisfied. This catches misconfiguration immediately at startup rather than discovering missing dependencies at runtime after you've already processed half your data. The fast-fail approach combined with clear error messages makes development rapid and safe.

# Protograph: MVP Definition

## MVP Scope

The Minimum Viable Product demonstrates Protograph's core value proposition: executing multi-step data processing pipelines with automatic dependency resolution, built-in retry logic, and clean testability. The MVP is intentionally minimal—pure in-memory synchronous execution with no persistence layer—to validate the programming model and developer experience before adding distributed systems complexity.

## Reference Implementation: Medical Test Result Interpretation

The MVP includes a complete working example that processes medical lab results through a five-step enrichment pipeline. Given a test result ID, the system fetches raw lab values, retrieves the patient's medical history, normalizes values for the patient's demographics, analyzes trends compared to historical results, and generates a clinical interpretation with recommendations. This example demonstrates real-world complexity: steps with single dependencies (fetch patient history from test result), steps with multiple dependencies (generate interpretation needs both normalized values and trend analysis), and derived information flowing through multiple transformation stages.

**Pipeline steps:**
- `FetchTestResult(testID)` → `RawTestResult` containing lab values and patient ID
- `FetchPatientHistory(rawResult)` → `PatientHistory` using patient ID from the raw result  
- `NormalizeValues(rawResult, patientHistory)` → `NormalizedResult` adjusted for patient demographics
- `CompareToBaseline(normalizedResult, patientHistory)` → `TrendAnalysis` showing how current values compare to patient's historical baseline
- `GenerateInterpretation(normalizedResult, trendAnalysis)` → `Interpretation` with clinical significance and recommendations

This pipeline naturally showcases dependency resolution (patient history depends on test result, normalization needs both, interpretation combines normalized values with trends), demonstrates why the system exists (medical results need fast processing for clinical use, pipeline logic changes frequently as medical knowledge evolves), and provides clear testability (mock the API client, verify each transformation step independently).

## Core Features

**Graph registration and validation.** Developers register steps at application startup using constructor-based dependency injection. Each step is a struct with a `Compute` method that declares proto dependencies through its function signature. The graph validates on startup that all dependencies can be resolved, no cycles exist, the one-step-per-output-type constraint holds, and all required runtime dependencies (database clients, API clients) have been provided. Validation errors are detailed and actionable, immediately catching configuration problems before any execution attempts.

**Automatic dependency resolution.** When execution is requested for an output type (e.g., `Interpretation`), the executor recursively walks the dependency graph, identifies which steps need to run and in what order, and executes them with resolved dependencies injected automatically. The developer never manually orchestrates step execution—they declare what they want, the system figures out how to produce it.

**Three-context model.** Graph-lifetime dependencies (API clients, database connections) are injected via step constructors and stored as struct fields, available throughout the step's existence. Execution-lifetime context (execution ID for tracking, standard Go context.Context for cancellation and timeouts) is passed as the first parameter to every Compute method. Proto dependencies (the actual data being transformed) are passed as subsequent typed parameters, resolved by the graph and injected automatically. This separation keeps infrastructure setup, execution tracking, and business logic cleanly separated.

**Retry logic with exponential backoff.** Steps can configure retry behavior at registration time: maximum attempts, backoff strategy (fixed, exponential, exponential with jitter), which error types are retriable. If a step fails with a retriable error, the executor automatically retries with the configured backoff before failing the entire execution. This handles transient failures (network blips, rate limits) without requiring explicit retry logic in business code.

**Full testability.** Every component has clean interfaces. Steps are easily mocked—create the step struct with mock clients, call Compute directly with test data, verify outputs. The graph can be tested in isolation—register steps, validate, inspect the dependency tree. Executors can be tested end-to-end—provide leaf inputs, verify the correct output is produced, confirm steps were called in the right order. No framework magic, no hidden state, just standard Go testing patterns.

## What's Explicitly Out of Scope

The MVP deliberately excludes: persistence and checkpointing (all execution is in-memory), asynchronous or queued execution (only synchronous execution that blocks until complete), job scheduling for future execution, worker pools and job distribution, rate limiting, circuit breakers, map/reduce operations over collections, admin UI or observability tooling beyond logging, and metrics collection. These features are important for production use but not required to validate the core programming model and developer experience.

# Protograph: MVP Definition

## MVP Scope

The Minimum Viable Product demonstrates Protograph's core value proposition: executing multi-step data processing pipelines with automatic dependency resolution, built-in retry logic, and clean testability. The MVP is intentionally minimal—pure in-memory synchronous execution with no persistence layer—to validate the programming model and developer experience before adding distributed systems complexity.

## Reference Implementation: Movie Content Tagging

The MVP includes a complete working example that processes movies through a five-step enrichment pipeline. Given a movie ID, the system fetches basic movie metadata, retrieves the director's filmography to understand their style, analyzes the movie's visual and audio content to extract themes, combines content analysis with directorial patterns to identify the movie's genre and mood, and generates a comprehensive set of searchable tags for discovery and recommendation. This example demonstrates real-world complexity: steps with single dependencies (fetch director filmography from movie metadata), steps with multiple dependencies (generate tags needs both content analysis and genre classification), and derived information flowing through multiple transformation stages.

**Pipeline steps:**
- `FetchMovie(movieID)` → `Movie` containing title, year, director ID, and basic metadata
- `FetchDirectorProfile(movie)` → `DirectorProfile` using director ID from the movie, includes their filmography and stylistic patterns
- `AnalyzeContent(movie)` → `ContentAnalysis` extracts visual themes, pacing, cinematography style from the movie itself
- `ClassifyGenreAndMood(contentAnalysis, directorProfile)` → `Classification` determines genre, subgenre, and emotional tone by combining content signals with directorial patterns
- `GenerateTags(movie, contentAnalysis, classification)` → `TagSet` produces comprehensive searchable tags combining metadata, content features, and classification

This pipeline naturally showcases dependency resolution (director profile depends on movie metadata, classification needs both content analysis and director style, tags combine all previous outputs), demonstrates why the system exists (content platforms need fast tagging for search and recommendations, tagging logic evolves as ML models improve), and provides clear testability (mock the video analysis API, verify each transformation independently).

## Core Features

**Graph registration and validation.** Developers register steps at application startup using constructor-based dependency injection. Each step is a struct with a `Compute` method that declares proto dependencies through its function signature. The graph validates on startup that all dependencies can be resolved, no cycles exist, the one-step-per-output-type constraint holds, and all required runtime dependencies (database clients, API clients) have been provided. Validation errors are detailed and actionable, immediately catching configuration problems before any execution attempts.

**Automatic dependency resolution.** When execution is requested for an output type (e.g., `TagSet`), the executor recursively walks the dependency graph, identifies which steps need to run and in what order, and executes them with resolved dependencies injected automatically. The developer never manually orchestrates step execution—they declare what they want, the system figures out how to produce it.

**Three-context model.** Graph-lifetime dependencies (API clients, database connections) are injected via step constructors and stored as struct fields, available throughout the step's existence. Execution-lifetime context (execution ID for tracking, standard Go context.Context for cancellation and timeouts) is passed as the first parameter to every Compute method. Proto dependencies (the actual data being transformed) are passed as subsequent typed parameters, resolved by the graph and injected automatically. This separation keeps infrastructure setup, execution tracking, and business logic cleanly separated.

**Retry logic with exponential backoff.** Steps can configure retry behavior at registration time: maximum attempts, backoff strategy (fixed, exponential, exponential with jitter), which error types are retriable. If a step fails with a retriable error, the executor automatically retries with the configured backoff before failing the entire execution. This handles transient failures (network blips, rate limits, video analysis service timeouts) without requiring explicit retry logic in business code.

**Full testability.** Every component has clean interfaces. Steps are easily mocked—create the step struct with mock clients, call Compute directly with test data, verify outputs. The graph can be tested in isolation—register steps, validate, inspect the dependency tree. Executors can be tested end-to-end—provide leaf inputs, verify the correct output is produced, confirm steps were called in the right order. No framework magic, no hidden state, just standard Go testing patterns.

## What's Explicitly Out of Scope

The MVP deliberately excludes: persistence and checkpointing (all execution is in-memory), asynchronous or queued execution (only synchronous execution that blocks until complete), job scheduling for future execution, worker pools and job distribution, rate limiting, circuit breakers, map/reduce operations over collections, admin UI or observability tooling beyond logging, and metrics collection. These features are important for production use but not required to validate the core programming model and developer experience.

# Protograph: Milestone Roadmap

## Milestone 1: Checkpointing and Crash Recovery

**Goal:** Add fault tolerance through PostgreSQL-based checkpointing while maintaining the in-memory fast path.

**Features:**
- Workflow-scoped checkpoint storage with `(workflow_id, output_type, input_hash)` composite keys
- Asynchronous persistence queue for non-blocking checkpoint writes
- In-memory cache with DB fallback on retry
- Checkpoint lookup logic: check memory first, then DB only on cache miss
- Automatic checkpoint cleanup after execution completes (1 hour TTL for abandoned executions)
- Execution lifecycle tracking (running, succeeded, failed states)

**Why this order:** Checkpointing is the foundation for all distributed features. Without it, you can't safely distribute work across multiple workers (if a worker crashes mid-execution, the work is lost). This milestone keeps execution synchronous and single-threaded but adds the durability layer needed for production use.

## Milestone 2: Asynchronous Job Queue

**Goal:** Enable async execution mode where requests are queued and processed by background workers.

**Features:**
- Job table with `SELECT FOR UPDATE SKIP LOCKED` pattern for distribution
- Submit job, get job ID immediately, execution happens asynchronously
- Job status tracking (queued, running, completed, failed)
- Worker pool that pulls jobs from queue and executes them
- Results stored and retrievable by job ID
- Both sync mode (Milestone 0) and async mode (this milestone) available

**Why this order:** With checkpointing in place, workers can safely crash and recover. Async mode enables high-throughput batch processing where response latency isn't critical (overnight enrichment jobs, scheduled reports, bulk data processing).

## Milestone 3: Scheduled Execution

**Goal:** Support jobs that run at specific times or on recurring schedules.

**Features:**
- `earliest_run_time` column on job table, workers only pick up jobs past their scheduled time
- Submit job with future execution timestamp
- Recurring job support (cron-style or interval-based)
- Lease-based pattern for recurring jobs to prevent duplicate execution
- Ability to cancel/update scheduled jobs before they execute

**Why this order:** Scheduled execution builds naturally on async queuing (Milestone 2). The same worker pool mechanism handles both immediate and future jobs. This enables use cases like "run this enrichment nightly" or "retry failed jobs after 1 hour" without external schedulers.

## Milestone 4: Fan-out and Map/Reduce

**Goal:** Process collections efficiently with automatic parallelization.

**Features:**
- Steps can accept slice inputs: `ProcessMovies(movies []Movie)` 
- Fan-out pattern: split collection, process items in parallel, collect results
- Map step: `EnrichMovie(movie Movie) -> EnrichedMovie` runs concurrently for each item
- Reduce step: `AggregateResults(enriched []EnrichedMovie) -> Summary`
- Individual item checkpointing (if 3/5 items cached, only compute 2)
- Configurable parallelism limits per step

**Why this order:** This is a significant complexity jump. With checkpointing (M1) and async workers (M2), you have the infrastructure to safely parallelize work. Map/reduce is a natural extension that dramatically expands what pipelines can express—batch processing becomes first-class.

## Milestone 5: Rate Limiting

**Goal:** Protect external services and control resource consumption.

**Features:**
- Per-step rate limits configured at registration
- Token bucket or sliding window algorithms
- Rate limit scopes: global (across all workers), per-execution, per-input-key
- When limit exceeded: queue for later (don't fail)
- Rate limit observability (current usage, time until tokens refill)

**Why this order:** Rate limiting makes most sense once you have async queuing (M2). In pure sync mode, rate limiting means blocking the caller, which defeats the purpose. With async mode, rate-limited jobs naturally wait in the queue until capacity is available.

## Milestone 6: Step Versioning and Cache Management

**Goal:** Handle implementation changes without corrupting cached data.

**Features:**
- Explicit version strings on steps: `StepVersion("v2.1.0")`
- Version included in checkpoint cache keys
- Automatic cache invalidation when version changes
- Optional TTL per step for time-sensitive data
- Cache warming: ability to pre-compute outputs for new versions
- Migration tooling: bulk recompute outputs after version bump

**Why this order:** In early milestones, you can just delete all checkpoints when you change code. As usage grows and checkpoint tables get large, you need smarter cache management. This milestone adds the primitives for handling code evolution gracefully.

## Milestone 7: Observability and Admin UI

**Goal:** Visibility into what's running, what's failing, and why.

**Features:**
- Execution tracing: record which steps executed, in what order, how long each took
- Execution visualization: show dependency graph for a completed execution
- Failed execution debugging: see which step failed, with full error details and context
- Queue monitoring: depth, throughput, worker utilization
- Step performance metrics: p50/p95/p99 latencies, success rates, retry rates
- Web UI for browsing executions, inspecting failures, manually retrying

**Why this order:** Early milestones focus on functionality; this milestone adds operational tooling. You need real production usage (which requires M1-M3) to know what observability is actually valuable. Building the UI too early means guessing at what operators need.

## Milestone 8: Circuit Breakers and Degraded Mode

**Goal:** Gracefully handle sustained failures without overwhelming downstream systems.

**Features:**
- Circuit breaker per step: track failure rates, open circuit after threshold
- When circuit open: fail fast without attempting execution (or return cached/default values)
- Automatic circuit recovery: half-open state, probe with single request
- Fallback values: steps can provide defaults if circuit is open
- Cascading circuit prevention: isolate failures to individual steps

**Why this order:** Circuit breakers are most valuable in high-throughput production systems where one failing dependency (external API down) can cascade and overwhelm everything. This requires the infrastructure from earlier milestones and usage patterns to tune thresholds.

## Milestone 9: Multi-tenancy and Resource Isolation

**Goal:** Run pipelines for multiple tenants with guaranteed isolation and fair resource sharing.

**Features:**
- Tenant ID as first-class concept in execution context
- Per-tenant rate limits and quotas
- Per-tenant worker pools (or weighted fair queuing)
- Tenant-scoped checkpoints and metrics
- Ability to disable/throttle specific tenants

**Why this order:** Multi-tenancy is an operational concern that only matters when you're running many different customers' workloads. The earlier milestones establish the single-tenant primitives; this milestone generalizes them.

## Milestone 10: Conditional Execution and Dynamic Graphs

**Goal:** Support pipelines where the dependency graph isn't fully known upfront.

**Features:**
- Conditional steps: execute step B only if step A's output meets condition
- Dynamic dependency declaration: step can request different dependencies based on its inputs
- Optional dependencies: step declares "I might need X, provide it if available"
- Multiple implementations per output type: choose step based on runtime conditions

**Why this order:** This fundamentally changes the "one output type, one step" constraint and requires careful design. You need extensive experience with the simpler model first to understand where flexibility is truly needed vs. where it just adds complexity.

---

## Future Additions (Long-term Roadmap)

**Advanced Execution Patterns:**
- Saga pattern with compensating transactions for multi-step rollback
- Long-running workflows that span hours/days with durable timers
- External event waiting (pause execution until external signal arrives)
- Human-in-the-loop steps (pause for approval, resume on decision)
- Streaming execution (step produces outputs incrementally, downstream steps process as available)
- Parallel branch execution with join synchronization (fork/join pattern)
- Sub-workflows (treat entire graphs as steps in larger graphs)

**Performance and Scalability:**
- Distributed caching layer (Redis) for hot checkpoints
- Checkpoint compression for large proto messages
- Batch job submission API (submit 1000 jobs with one DB round-trip)
- Priority queues (critical jobs skip ahead of normal jobs)
- Affinity scheduling (keep related jobs on same worker for cache locality)
- GPU worker pools for ML-heavy steps
- Result streaming for large outputs (avoid loading entire result in memory)

**Developer Experience:**
- Code generation from proto definitions to step skeletons
- Interactive graph visualization and exploration tool
- Step library/marketplace (common enrichment patterns, reusable components)
- Local development mode with instant reload on code changes
- Step profiling and optimization recommendations
- Automatic documentation generation from step definitions
- Integration with OpenTelemetry for distributed tracing
- Datadog/Prometheus metrics exporters

**Data Management:**
- Checkpoint archival to S3 for long-term storage
- Checkpoint replication across regions for disaster recovery
- Data lineage tracking (which inputs produced which outputs)
- GDPR compliance: right to be forgotten (delete all data for user ID)
- Audit logging (who requested what, when, with what inputs)
- Output versioning (keep multiple versions of outputs over time)
- Diff tooling (compare outputs from different step versions)

**Advanced Error Handling:**
- Partial failure modes (mark step as degraded but continue)
- Error aggregation (collect errors from fan-out operations, decide how to proceed)
- Automatic error classification using ML (transient vs permanent)
- Smart retry with jittered exponential backoff and deadline awareness
- Dead letter queues with automated analysis and replay
- Alerting integrations (PagerDuty, Slack) on sustained failures
- Error budgets (allow X% failures before circuit breaks)

**Testing and Validation:**
- Property-based testing framework for steps
- Chaos engineering mode (randomly inject failures to test resilience)
- Deterministic simulation testing (control time and randomness)
- Snapshot testing for step outputs (detect unintended changes)
- Integration test helpers (mock mode, assertion helpers)
- Load testing framework (simulate production traffic patterns)
- Mutation testing (verify tests actually catch bugs)

**Operational Features:**
- Blue/green deployments for graph changes
- Canary releases (route 5% of traffic to new version)
- Automatic rollback on error rate spike
- Traffic replay (re-execute historical jobs with new code)
- Cost attribution per tenant/pipeline/step
- Resource quotas and throttling policies
- Maintenance mode (drain workers gracefully)
- Backup and restore for checkpoint database

**Language and Integration Support:**
- Python SDK (call Protograph graphs from Python)
- TypeScript SDK for frontend/Node.js integration
- REST API for language-agnostic job submission
- GraphQL API for flexible querying
- Webhook callbacks on job completion
- Kafka integration (consume from topics, produce to topics)
- gRPC for high-performance inter-service calls
- AWS Lambda integration (run steps as Lambda functions)

**Advanced Graph Features:**
- Graph composition (combine multiple graphs)
- Cross-graph dependencies (output from graph A feeds graph B)
- Graph templates (parameterized graphs)
- Graph versioning and migrations
- A/B testing different graph implementations
- Feature flags to enable/disable steps
- Step mocking in production (replace real step with fake for testing)

**Security and Compliance:**
- Authentication and authorization per-graph/per-step
- Input validation and sanitization
- Output encryption for sensitive data
- API key rotation without downtime
- Secrets management integration (Vault, AWS Secrets Manager)
- Role-based access control for admin operations
- Audit trail for all configuration changes
- Compliance reporting (SOC2, HIPAA)

**AI and ML Integration:**
- Automatic step generation from natural language descriptions
- Intelligent retry scheduling based on historical patterns
- Anomaly detection for execution metrics
- Auto-tuning of rate limits and parallelism
- Predictive failure detection
- Automatic root cause analysis for failures
- Smart checkpoint eviction (ML-based cache replacement)
