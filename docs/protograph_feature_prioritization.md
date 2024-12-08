# Docket Feature Prioritization

This document summarizes the missing features identified by analyzing the Temporal Go SDK examples. Features are prioritized based on their frequency of use in common patterns and their necessity for building a functional and robust workflow engine.

## Priority Levels

*   **P0 (Critical):** Fundamental building blocks. Without these, a graph execution engine cannot function.
*   **P1 (High):** Essential features for building real-world, interactive, and reliable business processes.
*   **P2 (Medium):** Features needed for complex, long-running, or distributed patterns.
*   **P3 (Advanced/Niche):** Optimizations or specialized features for specific edge cases.

---

## P1: Essential Flow Control & Interactivity

These features transform a simple script runner into a workflow engine capable of handling concurrency, time, and external input.

*   **Concurrency / Parallelism:**
    *   **Race/Select:** Execute parallel steps and proceed as soon as *one* completes (cancellation of others). (Seen in: *Pick First, Timer*)
*   **Time Management:**
    *   **Durable Timers:** The ability to pause execution for a specific duration (sleep). (Seen in: *Timer, Sleep for Days, Updatable Timer*)
    *   **Timeouts:** Configurable timeouts for steps and the overall graph execution. (Seen in: *Activity Retries*)
*   **Interactivity (Signals & Updates):**
    *   **Signal Handling:** Ability to receive asynchronous events from external sources to change flow or state. (Seen in: *Await Signals, Shopping Cart, Safe Message Handler*)
    *   **Synchronous Updates:** A request/response pattern where an external caller waits for the graph to process an input and return a result. (Seen in: *Req/Resp Update, Shopping Cart*)
*   **Child Graphs:** The ability to compose graphs by calling one graph from another. (Seen in: *Child Workflow, PSO, Periodic Sequence*)
*   **Query API:** A mechanism to inspect the internal state of a running graph without affecting its execution. (Seen in: *Query, Shopping Cart, PSO*)

## P2: Robustness, Scale & Complex Patterns

Features required for production-grade systems, long-running processes, and distributed consistency.

*   **History Management (Looping):**
    *   **Continue-As-New:** A mechanism to restart a long-running graph with new state to prevent history bloat. Crucial for infinite loops or long-lived entities. (Seen in: *Sleep for Days, Shopping Cart, Safe Message Handler, PSO*)
*   **Advanced Error Handling:**
    *   **Compensation (Saga):** Native support or patterns for executing compensating steps to rollback distributed transactions upon failure. (Seen in: *Saga*)
    *   **Custom Retry Policies:** Fine-grained configuration of retries (backoff, max attempts, non-retryable errors). (Seen in: *Retry Activity, Infrequent Polling*)
*   **Observability:**
    *   **Distributed Tracing:** Integration with OpenTelemetry to trace execution across steps and services. (Seen in: *OpenTelemetry*)
    *   **Structured Logging:** Context-aware logging (injecting graph ID, step ID automatically). (Seen in: *Slog/Zap Adapters*)
*   **Resource Locality (Sessions):**
    *   **Worker Affinity:** Ensuring a sequence of steps runs on the same physical worker (e.g., for file processing). (Seen in: *File Processing, Worker-Specific Task Queues*)

## P3: Advanced Capabilities & Optimizations

Features that add significant power but are specific to certain architectural styles or optimizations.

*   **Dynamic/Generic execution:**
    *   **Dynamic Dispatch:** invoking steps or graphs by string name at runtime. (Seen in: *Dynamic, DSL*)
    *   **Generic Handlers:** "Catch-all" handlers for steps/graphs to implement dynamic routing or proxies. (Seen in: *Dynamic Workflows*)
*   **Scheduling:** Native cron-like scheduling for graphs. (Seen in: *Schedule, Cron*)
*   **Nexus (Service Abstraction):** Abstracting graph executions behind service API contracts for cross-boundary calls. (Seen in: *Nexus examples*)
*   **Security:**
    *   **Payload Encryption:** Transparent encryption of data at rest/in transit. (Seen in: *Encryption, Snappy Compression*)
    *   **Authorization/Policies:** Interceptors to enforce policies (e.g., blocking certain child graphs). (Seen in: *Security Interceptor*)
*   **Optimization:**
    *   **Eager Start:** Optimizing latency by starting execution on the local worker immediately. (Seen in: *Eager Start*)
    *   **Updatable Timers:** Modifying a sleep duration while it is in progress. (Seen in: *Updatable Timer*)

## Summary of Top 5 "Must-Haves" for MVP

Based on the breadth of examples, an MVP for Docket must prioritize:

1.  **Parallel Execution primitives:** Almost every non-trivial example uses some form of concurrency.
2.  **Signal/Event Listening:** Essential for any interactive or reactive workflow.
3.  **Durable Timers:** Fundamental for business processes (SLAs, delayed actions).
4.  **Composition (Child Graphs):** Key for code reuse and structuring complex logic.
5.  **Query/Update Capability:** Users need to interact with and inspect their running processes.
