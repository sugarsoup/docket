# Temporal Examples Analysis for Docket

This document analyzes a selection of Temporal examples and outlines the features Docket would need to replicate them.

## Hello World

**What it shows:** This is a basic Temporal application that demonstrates a simple workflow with a single activity. The workflow is triggered, executes an activity that takes a name and returns a greeting, and then the workflow completes, returning the greeting. It shows the basic structure of a Temporal application, including the client, worker, workflow, and activity.

**Docket features needed:**

*   **Graph Definition:** A way to define a graph that represents the workflow. This would likely involve defining steps that correspond to activities.
*   **Executor:** A component that can execute the graph. This would involve traversing the graph and executing the steps in the correct order.
*   **Activity/Step Implementation:** A way to define the logic for each step in the graph. In this case, a step that takes a string, and returns a new string.
*   **Client:** A way to start a graph execution and get the results.
*   **Worker:** A component that hosts the graph and step definitions and executes them.
*   **Basic Data Passing:** The ability to pass data (a string) into the workflow and get data (a string) out.

## Cron

**What it shows:** This example demonstrates how to schedule a workflow to run at a specific time or on a recurring basis, similar to a cron job. The workflow is scheduled to run every minute. It also shows how a workflow can access the result of its previous run, allowing it to maintain state across scheduled executions.

**Docket features needed:**

*   **Scheduler:** A component that can schedule graph executions based on a cron expression. This would likely be a separate component that interacts with the executor.
*   **Stateful Graphs:** A mechanism for a graph to access the results of its previous execution. This would allow for building stateful, recurring tasks. This could be implemented by passing the output of the previous run as an input to the next run.
*   **Graph Definition:** The graph definition would need to be able to specify that it can be scheduled and how to handle the state from the previous run.

## Child Workflow

**What it shows:** This example demonstrates how a workflow can start another workflow, known as a child workflow. The parent workflow starts a child workflow, passes it data, and then waits for the child workflow to complete. This allows for breaking down complex workflows into smaller, more manageable, and reusable parts.

**Docket features needed:**

*   **Graph Composition:** A way to define a graph that can execute another graph as one of its steps. This would be a special type of step that takes the ID of another graph to execute.
*   **Data Passing between Graphs:** The ability to pass data from the parent graph to the child graph and from the child graph back to the parent graph.
*   **Synchronous and Asynchronous Execution:** The ability for the parent graph to either wait for the child graph to complete (synchronous) or to continue its own execution without waiting (asynchronous). The example shows synchronous execution.

## Cancellation

**What it shows:** This example demonstrates how to cancel a running workflow. It also shows how a workflow can handle a cancellation request and perform cleanup operations before it terminates. A separate command is used to send the cancellation request to the running workflow. The workflow uses a `defer` block and a disconnected context to execute a cleanup activity when it receives a cancellation request.

**Docket features needed:**

*   **Cancellation API:** An API to request the cancellation of a running graph execution.
*   **Cancellation Handling:** A mechanism for a graph to detect that it has been canceled and to execute a specific set of cleanup steps before it terminates. This would likely involve a special "on cancel" handler in the graph definition.
*   **Disconnected Context:** The ability to execute cleanup steps in a context that is not itself canceled, so that they can run to completion.

## Query

**What it shows:** This example demonstrates how to query the state of a running workflow. A workflow can register a query handler to expose its internal state to an external client. This allows for monitoring the progress and state of a workflow without interrupting its execution.

**Docket features needed:**

*   **Query API:** An API to send a query to a running graph execution and receive a response.
*   **Query Handler:** A mechanism for a graph to register a handler for a specific query type. The handler would have access to the graph's internal state and could return it to the caller. This would likely be a special type of step or a property of the graph itself.
*   **State Management:** The graph needs a way to manage its internal state so that it can be accessed by the query handler.

## Update

**What it shows:** This example demonstrates the `update` feature, which allows for a synchronous, request-response interaction with a running workflow. The example implements a counter that can be atomically incremented. The `update` feature also allows for input validation before the update is even recorded in the workflow history.

**Docket features needed:**

*   **Update API:** An API to send an update request to a running graph execution and receive a response.
*   **Update Handler:** A mechanism for a graph to register a handler for a specific update type. The handler would have access to the graph's internal state, be able to modify it, and return a value to the caller.
*   **Input Validation:** The ability to define validation logic that is executed before an update is applied to the graph's state.
*   **Atomic State Modification:** The update handler must be able to modify the graph's state atomically.
*   **Signaling:** The example also uses a signal to terminate the workflow. Docket would need a way to send signals to a running graph.

## Await Signals

**What it shows:** This example demonstrates how to handle multiple signals that can arrive out of order, along with enforcing timeouts for their arrival. It uses a separate goroutine to listen for signals and update a shared state. The main workflow then uses `workflow.Await` and `workflow.AwaitWithTimeout` to wait for specific signals in sequence, respecting inter-signal and total signal reception timeouts. This pattern helps manage complex, asynchronous event coordination within a workflow.

**Docket features needed:**

*   **Signal Handling:** A robust mechanism to receive and process external signals during graph execution.
*   **Asynchronous Operations within Graph:** The ability to run parts of the graph (like a signal listener) in a concurrent manner (similar to goroutines).
*   **Shared State within Graph:** A way for different parts of a running graph (e.g., main execution flow and concurrent signal listeners) to safely share and update state.
*   **Conditional Awaiting:** A mechanism to pause graph execution until a specific condition (based on internal state or external events) becomes true, with optional timeouts. This would be analogous to `workflow.Await` and `workflow.AwaitWithTimeout`.
*   **Timers:** The ability to set and manage timers within the graph execution, leading to timeouts if conditions are not met within a specified duration.

## Batch Sliding Window

**What it shows:** This advanced example demonstrates a batch processing pattern using a "sliding window" of child workflows. A parent workflow (`SlidingWindowWorkflow`) orchestrates a configurable number of child workflows (`RecordProcessorWorkflow`) in parallel. Key aspects include:
*   **Dynamic Child Workflow Creation:** The parent dynamically starts child workflows.
*   **Resource Management:** It maintains a "sliding window" to limit concurrent child workflows, pausing execution until capacity is available.
*   **`continue-as-new` for Long-Running Workflows:** The parent uses `continue-as-new` to restart itself with updated state, preventing workflow history from growing too large.
*   **Inter-Workflow Communication (Signals):** Child workflows signal the parent upon completion, allowing the parent to track progress and manage the sliding window.
*   **Deterministic Non-Deterministic Operations:** Child workflows demonstrate using `workflow.SideEffect` for operations like generating random delays, ensuring determinism.
*   **External Monitoring:** The parent exposes its state via a query handler.
*   **Child Workflow Persistence:** Child workflows are configured to `Abandon` upon parent completion, ensuring they finish independently.
*   **Signal Draining:** Parent drains pending signals before `continue-as-new` to prevent data loss.

**Docket features needed:**

*   **Child Graph Execution with Resource Limits:** The ability for a graph to execute multiple instances of other graphs as child processes, with a mechanism to limit the number of concurrently running children (a sliding window).
*   **`Continue-As-New` Mechanism:** A feature to allow a graph to restart itself with a new input, effectively "resetting" its history to manage long-running processes.
*   **External Signals (from Children to Parent):** Graphs need to be able to send signals to other running graphs, specifically from a child graph back to its parent.
*   **Non-Deterministic Operation Handling:** A mechanism (like `workflow.SideEffect`) to allow for controlled execution of non-deterministic code within a deterministic graph environment.
*   **Graph Querying (Enhanced):** Beyond basic state queries, the ability to query specific aspects of a batch processing graph, such as current active children or completion progress.
*   **Child Graph Lifecycle Management:** Support for different parent-child relationship behaviors, like allowing child graphs to continue execution even if the parent completes (`ParentClosePolicy_Abandon`).
*   **Signal Buffering/Draining:** A way to manage signals when a graph is about to `continue-as-new`, ensuring no signals are lost during the transition.
*   **Advanced Scheduling/Concurrency Control:** More sophisticated control over how and when child graphs are started, based on available resources or other conditions.

## Branch

**What it shows:** This example demonstrates how to execute multiple activities (or steps) in parallel within a workflow and then wait for all of them to complete before continuing. This is a fundamental pattern for improving the efficiency of workflows by performing independent tasks concurrently.

**Docket features needed:**

*   **Parallel Step Execution:** A mechanism to define and execute multiple steps of a graph concurrently.
*   **Synchronization/Join:** A way to wait for the completion of all concurrently executed steps before the graph proceeds to subsequent steps. This would involve collecting the results of parallel branches.
*   **Dynamic Parallelism:** The ability to dynamically determine the number of parallel branches based on workflow input.

## Codec Server

**What it shows:** This example demonstrates the use of a custom `DataConverter` and a remote codec server to transparently encode and decode workflow payloads (inputs, outputs, signals, queries). This is particularly useful for scenarios requiring data encryption, compression, or transformation without modifying the core workflow logic. The example highlights:
*   **Transparent Data Conversion:** Workflow and activity code remains unaware of the encoding/decoding process, which is handled by the `DataConverter`.
*   **Remote Codec Service:** The actual encoding/decoding logic can reside in a separate service, allowing for centralized management of secrets (e.g., encryption keys) and easier deployment.
*   **Integration with Temporal CLI/UI:** The decoded payloads can be viewed in the Temporal CLI and UI by configuring them to use the remote codec endpoint.

**Docket features needed:**

*   **Pluggable Data Conversion:** A mechanism to allow users to define and plug in custom data converters for encoding and decoding graph inputs, outputs, and intermediate data. This would be configured at the executor/client level.
*   **Support for External Services:** The ability to integrate with external services (like a remote codec server) for specialized tasks that are not part of the core graph execution logic.
*   **Payload Security/Obfuscation:** Features that enable the encryption or obfuscation of sensitive data within graph payloads, without requiring changes to the graph definition itself.

## Context Propagation

**What it shows:** This example demonstrates how to propagate custom context (like tracing IDs, security tokens, or user-defined metadata) across workflow executions and activities. It utilizes Temporal's `ContextPropagator` interface, which allows for injecting and extracting values from the `context.Context` object, ensuring these values are available throughout the distributed workflow. The example also touches upon integration with tracing systems like Jaeger.

**Docket features needed:**

*   **Custom Context Propagation:** A mechanism to define and inject custom data into a graph's execution context, allowing this data to be propagated to all steps and sub-graphs.
*   **Observability Integration:** Built-in or extensible support for integrating with observability tools (like tracing systems) to provide end-to-end visibility of graph execution, including the propagated context.
*   **Context API:** A clear API for graph steps to access and utilize the propagated context values.

## Datadog Integration

**What it shows:** This example demonstrates how to integrate Temporal workflows with Datadog for comprehensive observability, including distributed tracing, custom metrics, and correlated logging. It highlights the use of Temporal's interceptor mechanism to automatically inject tracing and metrics collection into workflow and activity execution without modifying the core business logic.
*   **Distributed Tracing:** Uses Datadog's tracing interceptor to automatically capture and propagate trace contexts across workflows, child workflows, and activities.
*   **Custom Metrics:** Integrates with Prometheus (which Datadog can scrape) via `tally` to collect and report custom metrics from the workflow and worker.
*   **Correlated Logging:** Structured logging is configured to automatically include trace and span IDs, enabling easy correlation between logs and specific traces in Datadog.

**Docket features needed:**

*   **Observability Interceptors/Hooks:** A robust interception or hooking mechanism that allows external observability tools (like Datadog, Prometheus, Jaeger, etc.) to inject tracing, metrics collection, and contextual logging into graph execution.
*   **Metrics API:** A standardized way for graph steps to emit custom metrics that can be collected by integrated monitoring systems.
*   **Structured Logging with Context:** A logging framework that automatically enriches log entries with relevant execution context (e.g., graph ID, step ID, trace ID, span ID) to facilitate debugging and analysis in distributed systems.
*   **Integration with Third-Party Observability Tools:** A clear path or existing integrations for connecting Docket to popular observability platforms.

## Domain Specific Language (DSL) Workflow

**What it shows:** This example is highly relevant to Docket's core functionality. It demonstrates how to create a generic Temporal workflow that interprets a workflow definition provided externally, often in a declarative format like YAML. This allows for dynamic workflow creation and execution without recompiling application code. Key features include:
*   **Externalized Workflow Definitions:** Workflow logic is defined in a DSL (YAML in this case) rather than being hardcoded in Go.
*   **Interpreter Pattern:** A generic "DSL workflow" acts as an interpreter, parsing the external definition and executing the corresponding Temporal primitives (activities, sequences, parallel branches).
*   **Dynamic Activity Dispatch:** Activities are invoked dynamically based on names specified in the DSL.
*   **Control Flow (Sequence & Parallel):** The DSL supports both sequential execution of steps and parallel execution of branches, with mechanisms to handle failures in parallel branches.
*   **Data Flow:** A binding mechanism allows data to be passed between activities within the DSL-defined workflow.

**Docket features needed:**

*   **Declarative Graph Definition:** This is Docket's primary goal – to define workflows (graphs) using a declarative language. This example validates the need for a robust parsing and interpretation layer for such definitions.
*   **Graph Interpreter/Executor:** Docket would inherently be this component, capable of traversing a graph definition and executing its nodes (steps).
*   **Dynamic Step Execution:** The ability for the Docket executor to dynamically dispatch to different underlying implementations (activities) based on the step definition.
*   **Complex Control Flow Primitives:** Support for defining sequences of steps, parallel execution branches (fork/join), and conditional logic within the graph.
*   **Data Management within Graph:** A well-defined system for passing data between nodes, managing variables, and handling scope within the graph execution.
*   **Error Handling in Parallel Branches:** A mechanism to handle errors in concurrently executed graph branches, potentially including cancellation of other branches.

## Dynamic Invocation

**What it shows:** This example demonstrates the ability to invoke workflows and activities using their string names at runtime, rather than relying on strongly typed function references. This is a powerful feature for building flexible and dynamic systems where the exact workflow or activity to be executed might not be known until runtime, or when implementing generic executors for declarative workflow definitions.

**Docket features needed:**

*   **Dynamic Step Dispatch:** The core of Docket would involve dispatching to specific graph steps (which could map to activities or sub-graphs) based on their string identifiers, rather than compile-time function calls.
*   **Flexible Integration:** This dynamic invocation allows for easier integration with external systems or declarative configurations where step names are more practical than direct code references.
*   **Type Agnostic Execution (with serialization):** To support dynamic invocation, Docket would need robust serialization and deserialization mechanisms to pass data to and from dynamically invoked steps, as type information might not be available at compile time.

## Dynamic Workflows and Activities

**What it shows:** This example extends the concept of dynamic invocation by demonstrating how to register a single "dynamic" workflow or activity handler that can then internally dispatch to different logic based on the actual workflow or activity type requested. This allows for:
*   **Generic Handlers:** A single worker can handle a wide range of workflow and activity types without needing to register each one individually.
*   **Runtime Dispatch:** The dynamic handler can inspect the requested workflow/activity type at runtime and execute appropriate logic, potentially based on external configuration or a DSL.
*   **Flexible Deployment:** Reduces the need for workers to be tightly coupled to specific workflow implementations, making deployments more flexible.
*   **`converter.EncodedValues`:** Shows how to handle arguments generically when their specific type isn't known at compile time.

**Docket features needed:**

*   **Dynamic Workflow/Activity Registration:** Docket's core executor should be able to register generic handlers that can then interpret and execute various graph definitions or step implementations.
*   **Runtime Graph Interpretation:** The ability for the Docket engine to interpret and execute different graph structures or step logic based on runtime context or input.
*   **Generic Input/Output Handling:** A mechanism to handle inputs and outputs of graph steps in a type-agnostic way, likely relying on robust serialization/deserialization, similar to `converter.EncodedValues`.
*   **Pluggable Dispatch Logic:** The ability to customize how dynamic workflows/activities map to specific graph execution paths or step implementations.

## Dynamic mTLS Configuration

**What it shows:** This example demonstrates how to configure a Temporal client to use dynamically loaded mTLS (mutual TLS) certificates. This is crucial for applications that require high security and need to refresh their TLS credentials without restarting the client processes (workers or starters). The core idea is to load certificate files from disk and configure the client to watch for changes, enabling zero-downtime certificate rotation.

**Docket features needed:**

*   **Secure Communication Configuration:** Docket needs a robust mechanism to configure secure communication with its underlying infrastructure, including support for TLS/mTLS.
*   **Dynamic Credential Management:** The ability to dynamically load and refresh security credentials (like TLS certificates and keys) at runtime without requiring a restart of the Docket engine or workers.
*   **External Configuration Integration:** A way to integrate with external configuration sources or file systems to retrieve and monitor security-related files.
*   **Pluggable Security Providers:** The capability to plug in different security providers or mechanisms for handling authentication and encryption.

## Encryption

**What it shows:** This example demonstrates how to implement end-to-end encryption for workflow data (inputs, outputs, activity arguments, and results) using a custom `DataConverter` and `ContextPropagator`. This ensures that sensitive information is encrypted at rest within the Temporal server and in transit, while the workflow and activity logic itself remains agnostic to the encryption process. It highlights:
*   **Transparent Encryption:** Data is encrypted and decrypted automatically by the configured `DataConverter` without requiring changes to the workflow or activity code.
*   **Dynamic Key Management:** The example shows how an encryption `KeyID` can be passed via the workflow context (`ContextPropagator`), allowing for different encryption keys to be used per workflow execution.
*   **Security for Sensitive Data:** Provides a robust solution for handling sensitive information in a secure manner within durable workflows.
*   **Integration with Codec Server:** This feature works in conjunction with a codec server (as seen in the `codec-server` example) to allow for decryption and display of encrypted payloads in the Temporal CLI and UI.

**Docket features needed:**

*   **Pluggable Data Security Layer:** A configurable layer that allows for the injection of custom encryption/decryption logic for all data passing through the graph execution engine.
*   **Secure Context Propagation:** A mechanism to securely pass encryption metadata (like `KeyID`s) across graph steps and sub-graphs, ensuring consistent encryption policies.
*   **Policy-Driven Encryption:** The ability to define encryption policies (e.g., which data fields to encrypt, which keys to use) external to the graph definition.
*   **Data Masking/Redaction:** Beyond encryption, possibly features for masking or redacting sensitive data fields based on access controls during logging or display.

## Expense Report (Asynchronous Activity Completion)

**What it shows:** This example demonstrates a sophisticated pattern for integrating human interaction or external systems into a workflow using "asynchronous activity completion". An expense report workflow initiates, then pauses at an activity that waits for external approval. The activity doesn't complete immediately; instead, it returns a special status (`activity.ErrResultPending`), signaling to Temporal that its completion will be handled externally. An external system (simulated by a UI) then calls `client.CompleteActivity()` with a stored `TaskToken` to signal the activity's actual approval or rejection.
*   **Human-in-the-Loop:** Models scenarios where a workflow requires human intervention or external approval before proceeding.
*   **Asynchronous Activity Completion:** The core mechanism for an activity to return control to the worker without being marked as completed, awaiting an external callback.
*   **External System Integration:** Demonstrates how Temporal workflows can integrate seamlessly with external services that trigger activity completion.
*   **Callback Mechanism:** Uses `TaskToken` to uniquely identify the pending activity for external completion.

**Docket features needed:**

*   **Asynchronous Step Completion:** A mechanism for a graph step to indicate that its completion is pending an external event or callback, rather than completing synchronously within the step's execution.
*   **External Task Management:** Docket would need an API or process for external systems to notify it of the completion status of a pending graph step, possibly using a unique identifier (like `TaskToken`).
*   **Pause and Resume Capabilities:** The graph execution engine must be able to gracefully pause a graph at a specific step and resume it once the external completion signal is received.
*   **Human Task Integration:** Support for defining and managing "human tasks" within a graph, where a human actor's input is required for progression.
*   **Robust Error Handling for External Dependencies:** Mechanisms to handle timeouts, failures, or invalid external completion signals for pending steps.

## External Environment Configuration

**What it shows:** This example demonstrates how to configure Temporal clients (for both starters and workers) using external configuration sources, such as TOML files and environment variables. This approach promotes decoupling connection settings and other client-related configurations from the application's codebase, leading to more flexible and environment-agnostic deployments.
*   **Decoupled Configuration:** Client options (e.g., host, port, namespace, TLS settings) are defined outside the code.
*   **Hierarchical Configuration:** Supports loading configurations from files and overriding them with environment variables, providing a flexible precedence model.
*   **Simplified Deployment:** Enables easy adaptation of clients to different Temporal environments (development, staging, production) without code changes.
*   **`envconfig.LoadClientOptions`:** The Temporal SDK utility that facilitates loading these external configurations.

**Docket features needed:**

*   **External Configuration Management:** Docket's client and executor components should support loading configuration from external sources (e.g., files, environment variables, configuration services) to manage connection settings, authentication, and other operational parameters.
*   **Configurable Connection Options:** The ability to specify various connection details (endpoint, TLS, namespace) through configuration.
*   **Environment Agnostic Deployment:** Facilitate deploying Docket instances across different environments with minimal configuration changes.
*   **Configuration Overrides:** Support for programmatic or environment-variable-based overrides of configuration settings.

## File Processing (Session API)

**What it shows:** This example demonstrates the use of Temporal's "Session API" to ensure that a sequence of related activities executes on the same worker process. This is critical for scenarios where activities need to share local resources or maintain state on a specific machine, such as processing a file that was downloaded locally by a preceding activity.
*   **Activity Locality:** Guarantees that activities executed within a session will be dispatched to the same worker process, enabling shared access to local filesystems, memory caches, or other machine-local resources.
*   **Session Creation and Management:** Shows how to create a session (`workflow.CreateSession`), execute activities within its context, and complete it (`workflow.CompleteSession`).
*   **Failover and Retries:** Demonstrates how retrying a session-bound operation can lead to it being re-attempted on a different worker if the initial one fails, ensuring durability while maintaining locality within a new session.
*   **Stateful Activity Sequences:** Provides a pattern for chaining activities that rely on shared state or intermediate results stored locally.

**Docket features needed:**

*   **Step Locality Constraints:** A mechanism to specify that a sequence of graph steps must execute on the same worker/executor instance. This could involve grouping steps into a "session" or "affinity group."
*   **Shared Local Context:** Support for managing and accessing local, stateful contexts or resources that are shared among steps within a constrained locality.
*   **Session Management API:** Docket would need an API to create, manage, and complete such localized execution contexts.
*   **Fault Tolerance for Local Execution:** How to handle failures within a localized execution context, including retrying the entire local block on a different available instance.
*   **Resource Management:** Ability to specify resource requirements for workers that can host these localized execution contexts.

## Greetings Workflow

**What it shows:** This is a basic example demonstrating a sequential workflow composed of multiple activities. It illustrates how to define a series of steps where the output of one activity can be used as the input for the next. It also showcases a common pattern for organizing activity implementations as methods on a struct.
*   **Sequential Activity Execution:** Activities are executed in a defined order, one after another.
*   **Data Pipelining:** The output of `GetGreeting` and `GetName` activities are passed as inputs to the `SayGreeting` activity.
*   **Struct-based Activities:** Activities are implemented as methods on a Go struct (`Activities`), which can hold shared dependencies or configuration.
*   **Basic Workflow Structure:** A simple, foundational example for building more complex workflows.

**Docket features needed:**

*   **Sequential Step Definition:** The ability to define steps that execute in a specific, linear order.
*   **Data Flow between Steps:** A clear mechanism for passing outputs of one graph step as inputs to subsequent steps.
*   **Modular Step Implementation:** Support for organizing step logic (analogous to Temporal activities) in a modular way, potentially as methods of an object, allowing for dependency injection or shared state.
*   **Basic Graph Construction:** Foundational elements for constructing simple, linear graphs.

## Local Activities (Greetings Local)

**What it shows:** This example is a variation of the basic "Greetings Workflow," specifically demonstrating the use of "Local Activities." Local activities are a performance optimization where short-lived, idempotent activities are executed directly on the worker process that is currently executing the workflow task. This avoids the overhead of scheduling the activity on a task queue and a separate round-trip to the Temporal server.
*   **Performance Optimization:** Reduces latency and resource consumption for simple activities by eliminating task queue scheduling.
*   **Worker-Local Execution:** Activities run in the same process as the workflow logic, sharing the same memory space.
*   **`workflow.ExecuteLocalActivity`:** The specific API call used to invoke a local activity.
*   **`workflow.LocalActivityOptions`:** Configuration options for local activities, distinct from regular activities.

**Docket features needed:**

*   **Local Step Execution Mode:** The ability to designate certain graph steps as "local" (or "in-process") activities, allowing them to be executed directly by the graph executor without external scheduling.
*   **Performance Considerations:** Docket's architecture should support such optimizations for frequently used, low-latency steps.
*   **Configuration for Step Execution Location:** A mechanism to specify whether a step should be executed as a remote (task queue-based) or local activity.
*   **Resource Sharing for Local Steps:** If local steps share resources, Docket needs to ensure these are managed appropriately.

## gRPC Proxy

**What it shows:** This example demonstrates the use of a gRPC proxy to intercept and transform communication between Temporal clients/workers and the Temporal server. Specifically, it uses the proxy for transparent payload compression/decompression. This approach allows for centralizing cross-cutting concerns (like compression, encryption, or custom routing) at an infrastructure level, without modifying client or worker code.
*   **Transparent Payload Transformation:** The proxy automatically compresses outbound payloads and decompresses inbound payloads, making the process invisible to the client, worker, and workflow code.
*   **Centralized Codec Management:** The codec logic resides within the proxy, simplifying updates and management compared to configuring each client/worker individually.
*   **Infrastructure Layer for Cross-Cutting Concerns:** Shows how to use a proxy to inject capabilities like security (e.g., authorization, authentication) or observability at the communication layer.
*   **gRPC Interceptors:** Leverages gRPC client and server interceptors for modifying request/response flows.

**Docket features needed:**

*   **Pluggable Communication Layer:** Docket should have a flexible architecture that allows for the insertion of custom proxies or interceptors to modify communication with its backend or other services.
*   **Transparent Data Handling:** Similar to codecs and encryption, Docket needs mechanisms to handle data transformations (e.g., compression, serialization) transparently to the graph definition.
*   **Centralized Policy Enforcement:** The ability to enforce policies (security, QoS, data transformation) at a central point that affects all graph executions.
*   **Network Abstraction:** Provide a layer of abstraction over network communication to allow for different underlying transport mechanisms or proxies.

## Hello World with API Key Authentication

**What it shows:** This example is an extension of the basic "Hello World" workflow, focusing on client authentication using an API key. It demonstrates how to configure a Temporal client (for both the starter and worker) to connect to a Temporal service that requires API key authentication. This is particularly relevant when interacting with managed Temporal services (like Temporal Cloud) where API keys are a common authentication method.
*   **API Key Authentication:** Shows how to pass an API key to the Temporal client for secure connection.
*   **Secure Client Configuration:** Illustrates how to configure `client.Options` to include authentication details.
*   **Deployment with Security:** Provides a practical example of setting up a secure client-server interaction.

**Docket features needed:**

*   **Authentication Mechanisms:** Docket's client components need to support various authentication methods, including API keys, for connecting to the underlying workflow orchestration engine.
*   **Configurable Security Credentials:** The ability to specify and manage security credentials (like API keys) through configuration, environment variables, or other secure means.
*   **Secure Connection Setup:** Ensure that client-server communication can be secured using appropriate authentication.

## Hello World with mTLS

**What it shows:** This example demonstrates how to configure a Temporal client (for both the starter and worker) to establish a secure connection with the Temporal service using mutual TLS (mTLS). This is a higher level of security than API key authentication, as both the client and server authenticate each other using cryptographic certificates.
*   **mTLS Configuration:** Shows how to load client certificates and keys to configure the Temporal client for mTLS.
*   **Two-Way Authentication:** Ensures both the client and server verify each other's identity, preventing unauthorized access and man-in-the-middle attacks.
*   **Secure Communication Channel:** All communication between the client/worker and the Temporal server is encrypted.

**Docket features needed:**

*   **mTLS Support:** Docket's client components must support mTLS for secure communication with the workflow orchestration engine, allowing for robust mutual authentication.
*   **Certificate Management:** A mechanism to load and manage TLS certificates and private keys for client authentication.
*   **Secure Configuration Practices:** Guidelines and tooling for securely providing and managing mTLS credentials.

## Metrics Integration (Prometheus)

**What it shows:** This example demonstrates how to integrate Temporal workers with a metrics system, specifically Prometheus, using the `tally` library. It illustrates how to configure a worker to expose metrics via a Prometheus endpoint, enabling monitoring of workflow and activity execution, and allowing for the creation of custom metrics.
*   **Prometheus Metrics Exposure:** The worker is configured to expose a `/metrics` endpoint that can be scraped by Prometheus.
*   **`tally` Library Integration:** Uses the `tally` library for metric definition and reporting, which is integrated with the Temporal SDK's `MetricsHandler`.
*   **Custom Metrics:** Shows how to emit custom metrics (e.g., activity start times, durations) from within activities.
*   **Workflow and Activity Observability:** Provides a foundation for monitoring the performance and health of Temporal applications.

**Docket features needed:**

*   **Pluggable Metrics System:** A flexible system that allows Docket to emit metrics in a format compatible with popular monitoring solutions (e.g., Prometheus, Datadog, StatsD).
*   **Metrics API for Steps:** An API for graph steps to emit custom metrics (counters, gauges, timers) to track business-specific events or performance.
*   **Automatic Metric Collection:** Docket should automatically collect core execution metrics (e.g., graph execution time, step duration, retry counts).
*   **Tagging/Labeling:** Support for attaching meaningful tags or labels to metrics for easier aggregation and filtering in monitoring systems.
*   **Integration with Observability Platforms:** Clear mechanisms for connecting Docket's metrics output to external monitoring dashboards and alerting systems.

## Multi-History Replay

**What it shows:** This example demonstrates the powerful capability of replaying workflow execution histories. This feature is fundamental for debugging, auditing, testing new code against past executions, and migrating workflows across different environments or code versions. It involves retrieving existing workflow histories from the Temporal server and then re-executing the workflow logic locally using a `WorkflowReplayer`.
*   **History Retrieval:** Shows how to query and fetch workflow execution histories.
*   **Local Replay:** The `WorkflowReplayer` allows for deterministic re-execution of a workflow's logic using its historical events, without interacting with external activities.
*   **Debugging and Auditing:** Essential for understanding how a workflow reached a particular state or for verifying its behavior.
*   **Version Compatibility Testing:** Enables testing code changes or new workflow definitions against production histories to ensure backward compatibility.

**Docket features needed:**

*   **Graph History Access:** Docket would need mechanisms to access and retrieve the historical execution data of a graph.
*   **Deterministic Replay Engine:** A core capability to deterministically re-execute a graph's logic from its historical events, allowing for debugging, auditing, and testing.
*   **Snapshotting/Checkpointing:** To optimize replay, potentially integrate with mechanisms to save and restore graph states.
*   **Version Control for Graphs:** Replay capabilities are often linked with versioning of graph definitions to ensure that an old history can be replayed against the correct version of the graph logic.
*   **Isolation of External Interactions:** During replay, external interactions (like activity calls) should be mocked or replayed from recorded results to maintain determinism.

## Logger Interceptor

**What it shows:** This example demonstrates how to use Temporal's worker interceptors to customize the logging behavior within workflows and activities. Interceptors allow you to inject custom logic for cross-cutting concerns without modifying the core workflow or activity code. In this case, a custom interceptor is used to automatically add `WorkflowStartTime` and `ActivityStartTime` tags to all log entries generated by `workflow.GetLogger(ctx)` and `activity.GetLogger(ctx)`.
*   **Custom Logging Context:** Enriches log messages with additional, automatically injected context.
*   **Decoupled Logging Concerns:** Separates logging customization from business logic.
*   **Worker Interceptors:** Shows how to implement and register a `WorkerInterceptor` to modify the behavior of workflows and activities executed by that worker.
*   **Observability Customization:** Provides a powerful mechanism for tailoring observability (logging) to specific needs.

**Docket features needed:**

*   **Pluggable Logging System:** A flexible logging framework that allows for custom log appenders, formatters, and contextual enrichment.
*   **Execution Lifecycle Hooks/Interceptors:** A mechanism (similar to Temporal's interceptors) to inject custom logic at various points in a graph's execution lifecycle (e.g., before/after step execution, before/after logging), enabling cross-cutting concerns like logging, metrics, and tracing.
*   **Contextual Logging:** Automatically enriching log messages with graph-specific context (e.g., graph ID, step ID, execution start times).
*   **Integration with External Logging Systems:** Easy integration with external log management platforms.

## Workflow Memos

**What it shows:** This example demonstrates the use of "Memos" in Temporal workflows. Memos are dynamic, searchable key-value pairs that can be attached to a workflow execution. They are useful for storing metadata or brief descriptions that are easily accessible via the Temporal CLI or UI, without needing to query the workflow's internal state.
*   **Attaching Metadata:** Shows how to associate arbitrary data with a workflow at the time of its creation.
*   **Dynamic Updates:** Memos can be updated dynamically during the workflow's execution using `workflow.UpsertMemo`.
*   **Visibility and Search:** The data stored in memos is typically indexed by the Temporal service, making it searchable and visible in the Temporal UI/CLI.
*   **Workflow Inspection:** Demonstrates how a workflow can retrieve its own memo fields.
*   **External Verification:** An activity is used to query the Temporal service's visibility store to confirm memo updates.

**Docket features needed:**

*   **Graph Metadata:** A mechanism to attach user-defined key-value metadata to a graph execution, which can be dynamically updated.
*   **Queryable Metadata:** This metadata should be easily queryable via a CLI, UI, or API for monitoring and management purposes.
*   **Dynamic Metadata Updates:** The ability for a running graph to update its own metadata.
*   **Visibility Integration:** Docket's runtime should provide a way to expose this metadata through its management interfaces.
*   **Contextual Data Storage:** Differentiate between transient data passed between steps and persistent metadata associated with the entire graph execution.

## Distributed Mutex

**What it shows:** This advanced example demonstrates how to implement a distributed mutex using Temporal workflows to coordinate access to shared resources across multiple, concurrently running workflows. This pattern is crucial for preventing race conditions and ensuring mutually exclusive operations in a distributed system.
*   **Inter-Workflow Communication (Signals):** Workflows communicate by sending and receiving signals to acquire and release locks.
*   **Centralized Lock Management:** A dedicated "Mutex Workflow" manages the state of the locks for various resources.
*   **Blocking for Resource Access:** Client workflows block their execution (`workflow.Receive` on a signal channel) until they acquire a lock.
*   **Conditional Logic in Workflow:** The Mutex Workflow uses conditional logic to grant locks and wait for releases.
*   **`SignalWithStartWorkflow`:** Used by client workflows to interact with the Mutex Workflow, ensuring it's started if not already running, and immediately sending a lock request.
*   **Determinism with `workflow.SideEffect`:** Used to generate unique channel names for releasing locks in a deterministic manner.

**Docket features needed:**

*   **Distributed Coordination Primitives:** Docket would need native support for distributed coordination primitives like mutexes, semaphores, or locks to manage shared resources across different graph executions.
*   **Inter-Graph Communication:** A robust mechanism for different running graphs to communicate with each other, for instance, through signals or shared state that can trigger actions.
*   **Blocking/Waiting on External Events:** The ability for a graph step to pause its execution and wait for an external event or signal (like a lock acquisition) before proceeding.
*   **Stateful Coordination Workflows:** Support for defining graphs that manage and orchestrate shared state or resources for other graphs.
*   **External Signal Triggering:** The ability for a graph to send signals to other specific graph instances.
*   **Guaranteed Delivery of Coordination Messages:** Ensuring that messages (signals) used for coordination are reliably delivered.

## Temporal Nexus

**What it shows:** This advanced and highly significant example introduces "Temporal Nexus," a feature designed to simplify connecting durable executions across various boundaries (teams, namespaces, regions, clouds). It enables building modular, service-oriented architectures by exposing well-defined service API contracts that abstract away underlying Temporal primitives or execute arbitrary code.
*   **Service-Oriented Architecture:** Defines clear API contracts for services (`service/api.go`) that can be implemented and consumed across different Temporal domains.
*   **Nexus Client:** Workflows can act as Nexus clients, calling operations on remote Nexus services using `workflow.NewNexusClient` and `c.ExecuteOperation`.
*   **Nexus Handler (Workflow-Backed):** Nexus operations can be implemented by starting Temporal workflows (`temporalnexus.NewWorkflowRunOperation`), abstracting the workflow execution detail from the caller.
*   **Nexus Handler (Synchronous):** Nexus operations can also be implemented by simple synchronous functions (`nexus.NewSyncOperation`) for immediate RPC-like responses.
*   **Request Deduplication:** Utilizes `options.RequestID` to ensure that even if an operation is called multiple times, the underlying workflow (if any) is only started once.
*   **Abstraction Layer:** Nexus provides a powerful abstraction over Temporal's core primitives, allowing users to interact with durable services without needing deep knowledge of workflows, activities, or signals.

**Docket features needed:**

*   **Service Abstraction Layer:** Docket needs a mechanism to define and consume abstract "services" or "functions" that can be implemented by various underlying execution models (e.g., other graphs, external microservices).
*   **Cross-Graph Communication (Nexus-like):** The ability for one graph to call an operation on another graph (or external service) with strong API contracts, potentially across different Docket instances or deployments.
*   **Operation Definition and Implementation:** Support for defining operations (with inputs and outputs) and then associating these operations with specific graph patterns or external logic for their implementation.
*   **Request Deduplication for Operations:** A built-in mechanism to prevent duplicate execution of operations if they are called multiple times due to retry logic or network issues.
*   **Synchronous and Asynchronous Operation Execution:** The ability to support both immediate-response operations and operations that initiate longer-running, durable processes (like a workflow).
*   **API for External Integration:** A clear and well-defined API for Docket services to be consumed by external clients.

## Search Attributes

**What it shows:** This example demonstrates how to leverage Temporal's Search Attributes for querying and filtering workflow executions based on custom business-specific metadata. It covers defining custom search attributes of various types, setting and updating their values within a workflow, and then querying these attributes from an external client.
*   **Custom Search Attribute Definition:** Shows how to define custom search attributes with different data types (e.g., `Int64`, `Keyword`, `Bool`, `Double`, `Datetime`, `KeywordList`).
*   **Setting Search Attributes:** Workflows can set initial search attribute values when they are started.
*   **Dynamic Search Attribute Updates (`UpsertTypedSearchAttributes`):** Workflows can dynamically update their search attributes during execution, allowing their searchable metadata to evolve with their state.
*   **Unsetting Search Attributes:** Shows how to remove search attributes.
*   **Querying Workflow Executions:** Demonstrates how to query the Temporal visibility store for workflow executions using complex queries based on search attributes.
*   **Eventual Consistency Handling:** The example accounts for the eventual consistency of the visibility store, waiting for updates to be indexed before querying.

**Docket features needed:**

*   **Graph Metadata Management:** The ability to associate user-defined, searchable metadata with graph executions.

## Session Failure Recovery

**What it shows:** This example demonstrates how a Temporal workflow can gracefully recover from the failure of a session worker. When an activity running within a session is hosted on a worker that crashes or becomes unavailable, the session fails. The workflow detects this failure and then attempts to restart the entire sequence of session-bound activities on a new session, effectively on a different worker.
*   **Session-Bound Activities:** Activities are executed within a session, ensuring they run on the same worker for a period.
*   **Session Failure Detection:** The workflow monitors the session state and detects when a session fails due to worker unavailability.
*   **Workflow-Driven Recovery:** Upon session failure, the workflow explicitly retries the session creation and execution, orchestrating the recovery process.
*   **Timeouts and Retries:** Uses `workflow.Sleep` to introduce delays before retrying a failed session, and activity options for activity-specific timeouts.
*   **Fault Tolerance for Localized State:** Provides a pattern for building highly available systems even when some operations require locality to a specific worker.

**Docket features needed:**

*   **Session/Locality Management:** Docket needs robust mechanisms for defining and managing localized execution contexts (sessions) for a sequence of steps.
*   **Fault Tolerance for Local Execution:** The ability for graphs to detect failures within a local execution context and implement recovery strategies, such as retrying the entire local block.
*   **Configurable Recovery Policies:** Flexible policies for how graphs should react to step or session failures, including retry delays and maximum attempts.
*   **Workflow State Persistence across Retries:** Ensuring that necessary state is preserved and correctly passed to new attempts of a failed local execution.
*   **Resource Affinity:** The ability to specify that a group of steps should ideally run on the same executor instance.

## Structured Logging (slog Adapter)

**What it shows:** This example demonstrates how to integrate Go's new structured logging library (`log/slog`) with Temporal workflows and activities. It highlights how to configure the Temporal client and worker to use a custom `slog` logger, ensuring that all log messages generated within the workflow execution are structured, easily machine-readable (JSON format), and automatically enriched with relevant Temporal context (e.g., WorkflowID, RunID, ActivityID, TaskQueue).
*   **Structured Logging:** All log output is in JSON format, making it easier to parse and analyze with log aggregation tools.
*   **Contextual Logging:** Temporal-specific metadata (workflow, activity, and worker details) is automatically added to each log entry, providing rich context for debugging and observability.
*   **`slog` Integration:** Shows how to wrap an `slog` handler with Temporal's `tlog.NewStructuredLogger` to provide a unified logging experience.
*   **Clean Separation of Concerns:** The core workflow and activity logic continues to use `workflow.GetLogger(ctx)` and `activity.GetLogger(ctx)`, remaining decoupled from the specific logging implementation.

**Docket features needed:**

*   **Pluggable/Configurable Logging Framework:** A flexible logging system that allows users to plug in their preferred structured logging libraries and handlers.
*   **Automatic Contextual Data Injection:** Automatically enrich log messages with graph-specific metadata (e.g., graph instance ID, step ID, execution attempts, parent graph IDs).
*   **Structured Output Formats:** Support for various structured log output formats (e.g., JSON) to facilitate integration with log management systems.
*   **Unified Logging Interface:** Provide a consistent logging API within graph steps that developers can use, regardless of the underlying logging implementation.
*   **Configurable Log Levels:** Ability to set different log levels for various components of the graph execution.

## Updatable Timer

**What it shows:** This example demonstrates how to implement an "updatable timer" within a Temporal workflow. This pattern allows a workflow to pause its execution for a specified duration, but crucially, this duration can be modified (rescheduled) dynamically by external signals while the timer is still pending. This is valuable for scenarios where delays need to be adjusted based on real-time events.
*   **Dynamic Delay Adjustment:** An external signal can dynamically change the wake-up time of a pending timer.
*   **Timer Cancellation and Rescheduling:** When an update signal is received, the current timer is canceled, and a new timer is created with the updated wake-up time.
*   **`workflow.NewSelector` for Event Handling:** A `workflow.Selector` is used to concurrently wait for either the timer to fire or an update signal to arrive.
*   **External Control over Delays:** Enables external clients to influence the timing of workflow events.
*   **Durable Pauses:** The timer mechanism is durable, ensuring the workflow will resume at the correct time even after worker restarts.

**Docket features needed:**

*   **Updatable Delay Steps:** The ability to define steps that introduce delays, where the duration of these delays can be dynamically modified by external events (e.g., signals, external API calls).
*   **Dynamic Scheduling of Future Actions:** Support for dynamically rescheduling future actions or pauses within a graph.
*   **Event-Driven Delay Management:** A mechanism to combine external event listeners with delay mechanisms to create dynamic pauses.
*   **Cancellation of Pending Delay Operations:** The ability to cancel existing delay operations and replace them with new ones.
*   **Query Current Delay Status:** The ability to query the remaining time or the scheduled wake-up time of a pending delay.

## Worker Versioning

**What it shows:** This example demonstrates Temporal's "Worker Versioning" feature, a critical capability for safely deploying updates to workflow and activity code in production environments. It allows you to introduce new versions of your workflow logic while ensuring that existing long-running workflows continue to execute correctly, and new workflows are scheduled on compatible workers.
*   **Safe Code Deployment:** Enables rolling out code changes without impacting in-flight workflows or requiring all workers to be updated simultaneously.
*   **Auto-Upgrading Workflows:** Workflows configured with `VersioningBehaviorAutoUpgrade` automatically transition to the newest compatible worker version as it becomes available.
*   **Pinned Workflows:** Workflows configured with `VersioningBehaviorPinned` remain associated with the specific worker version they started on, ensuring their continued execution even if newer, incompatible worker versions are deployed.
*   **Compatible Changes with `workflow.GetVersion`:** Demonstrates how to introduce compatible changes to workflow logic using `workflow.GetVersion`, allowing workflows to adapt their behavior based on the worker version they are executing on.
*   **Incompatible Changes Handling:** Illustrates scenarios where workflow or activity signatures change in an incompatible way, and how worker versioning helps manage these transitions for pinned workflows.
*   **Deployment Options:** Uses `worker.DeploymentOptions` to define `BuildID`s for different worker versions.

**Docket features needed:**

*   **Graph Versioning:** Docket needs a robust system for versioning graph definitions, allowing for safe and controlled deployment of changes.
*   **Deployment Strategies (Rolling Updates, Canary):** Support for various deployment strategies that leverage versioning to minimize risk, including automatically migrating (auto-upgrading) or isolating (pinning) running graph instances to specific versions.
*   **Compatibility Management:** Mechanisms to detect and handle compatible and incompatible changes in graph definitions and step implementations.
*   **Dynamic Logic Adaptation:** The ability for graph logic to adapt its execution path or behavior based on its current version or the version of the executor it's running on.
*   **Rollback Capabilities:** Versioning enables easier rollback to previous stable versions of a graph.
*   **Worker/Executor Affinity to Versions:** A way to bind specific executor instances (or pools) to particular versions of graph definitions.

## Workflow Security Interceptor (Child Workflow Type Validation)

**What it shows:** This example demonstrates how to implement a worker interceptor to enforce security policies on child workflow creation. It showcases how to intercept requests to start child workflows and validate their types, preventing the execution of unauthorized or "prohibited" child workflows. This is a powerful mechanism for building secure, multi-tenant Temporal applications where certain workflow types might be restricted.
*   **Worker Interceptors for Security:** A `WorkerInterceptor` is used to inject custom logic into the child workflow creation process.
*   **Child Workflow Type Validation:** The interceptor calls an activity (`ValidateChildWorkflowTypeActivity`) to check if the requested child workflow type is allowed.
*   **Policy Enforcement:** Demonstrates how to enforce policies at the platform level, preventing certain workflow patterns or types from being executed.
*   **Custom Error Handling:** The interceptor can return a custom error if a child workflow type is not allowed, leading to a controlled workflow failure.

**Docket features needed:**

*   **Policy Enforcement Layer:** A mechanism to define and enforce security policies (e.g., allowed graph types, allowed step types, access control) at various points in the graph execution lifecycle.
*   **Graph Interceptors/Hooks:** The ability to inject custom validation logic before a child graph is executed or a step is invoked.
*   **Declarative Security Policies:** A way to define security policies (e.g., allowlist/blocklist of graph types) in a declarative manner.
*   **Pre-Execution Validation:** Support for validating graph structure or execution requests before the graph actually starts.
*   **Centralized Security Configuration:** Manage security policies centrally, decoupled from individual graph definitions.

## Structured Logging (zap Adapter)

**What it shows:** This example demonstrates how to integrate the popular Uber `zap` structured logging library with Temporal workflows and activities. Similar to the `slogadapter` example, it highlights configuring the Temporal client and worker to use a custom `zap` logger, ensuring that all log messages generated within the workflow execution are structured, machine-readable (JSON format), and automatically enriched with relevant Temporal context (e.g., WorkflowID, RunID, ActivityID, TaskQueue).
*   **Structured Logging with `zap`:** All log output is in JSON format, leveraging `zap`'s performance and structured logging capabilities.
*   **Contextual Logging:** Temporal-specific metadata is automatically added to each log entry, providing rich context for debugging and observability.
*   **`zap` Integration:** Shows how to wrap a `zap.Logger` with a custom adapter (`zapadapter.NewZapAdapter`) to provide a unified logging experience within Temporal.
*   **Clean Separation of Concerns:** The core workflow and activity logic continues to use `workflow.GetLogger(ctx)` and `activity.GetLogger(ctx)`, remaining decoupled from the specific `zap` implementation.

**Docket features needed:**

*   **Pluggable/Configurable Logging Framework:** A flexible logging system that allows users to plug in their preferred structured logging libraries (like `zap`) and handlers.
*   **Automatic Contextual Data Injection:** Automatically enrich log messages with graph-specific metadata (e.g., graph instance ID, step ID, execution attempts, parent graph IDs).
*   **Structured Output Formats:** Support for various structured log output formats (e.g., JSON) to facilitate integration with log management systems.
*   **Unified Logging Interface:** Provide a consistent logging API within graph steps that developers can use, regardless of the underlying logging implementation.
*   **Configurable Log Levels:** Ability to set different log levels for various components of the graph execution.

## Worker-Specific Task Queues

**What it shows:** This example demonstrates an alternative approach to "Sessions" for ensuring a sequence of activities runs on a single, specific worker process. This is particularly useful for tasks that require significant local state or resource affinity, like processing a downloaded file on the same host where it resides.
*   **Dynamic Task Queue Generation:** Each worker process creates and listens on a unique, worker-specific task queue (e.g., using a UUID).
*   **Shared and Worker-Specific Task Queues:** Workflows start on a shared task queue. An initial activity then determines the unique task queue of the worker that executed it. Subsequent activities requiring locality are then scheduled on this worker-specific task queue.
*   **Resource Affinity:** Guarantees that related activities (e.g., file download, processing, and deletion) execute on the same physical worker, enabling shared access to local disk or memory.
*   **Fault Tolerance with Retries:** If a worker goes down, the workflow detects the failure (via activity timeouts on the worker-specific task queue) and can retry the entire sequence on a *new* worker, which will then generate its own unique task queue.
*   **Fine-Grained Control:** Offers more manual control over worker assignment compared to the Session API.

**Docket features needed:**

*   **Custom Task Queue Allocation:** The ability to dynamically assign custom task queues to specific groups of graph steps or individual graph instances.
*   **Resource Affinity Control:** Mechanisms to explicitly specify that a set of graph steps should be executed on the same physical executor instance, potentially for local resource sharing.
*   **Dynamic Executor Identification:** A way for a graph step to identify the unique identifier of the executor instance it's running on, to facilitate routing subsequent steps.
*   **Fault-Tolerant Local Sequences:** Robust error handling and retry mechanisms for sequences of steps that rely on local resources, allowing them to restart on new executor instances.
*   **Comparison to Sessions:** Evaluate the trade-offs between a "Session" equivalent and a "Worker-Specific Task Queue" equivalent for managing locality in Docket.

## Snappy Compression

**What it shows:** This example demonstrates how to transparently apply Snappy compression to workflow payloads (inputs, outputs, activity arguments, and results) using a custom `DataConverter`. Similar to the `codec-server` and `encryption` examples, the core idea is to decouple data transformation concerns from the business logic of the workflow.
*   **Transparent Compression:** Payload data is automatically compressed and decompressed by the `DataConverter`, making the process invisible to the workflow and activity code.
*   **Reduced Data Transfer:** Compression helps reduce the amount of data transferred between clients, workers, and the Temporal server, which can improve performance and reduce network costs, especially for workflows dealing with large payloads.
*   **Custom `DataConverter` for Compression:** Highlights the implementation of a custom `DataConverter` that utilizes the Snappy compression algorithm.
*   **Integration with Codec Server:** This feature works in conjunction with a codec server to allow for decompression and display of compressed payloads in the Temporal CLI and UI.

**Docket features needed:**

*   **Pluggable Data Transformation:** A configurable layer that allows for the injection of custom data transformation logic (e.g., compression, encryption, serialization/deserialization) for all data passing through the graph execution engine.
*   **Payload Optimization:** Mechanisms to optimize the size of data payloads transferred between graph steps, including transparent compression options.
*   **Declarative Data Handling Policies:** The ability to declare policies (e.g., compression algorithms, encryption methods) for data handling, separate from the core graph logic.
*   **Performance Considerations for Data Transfer:** Docket's architecture should consider and allow for optimizations related to data transfer efficiency.

## Split-Merge (Ordered Futures)

**What it shows:** This example demonstrates a common distributed computing pattern called "split-merge" or "fan-out/fan-in". It involves executing multiple independent tasks (activities) in parallel and then collecting and aggregating their results. This specific example uses `workflow.Future` objects and retrieves results in the order the activities were invoked.
*   **Parallel Activity Execution:** Multiple `ChunkProcessingActivity` instances are launched concurrently within a loop.
*   **`workflow.Future` for Asynchronous Results:** Each call to `workflow.ExecuteActivity` immediately returns a `workflow.Future`, representing the eventual result of the activity.
*   **Ordered Result Merging:** The workflow explicitly iterates through the `workflow.Future` slice and calls `Get(ctx, &result)` on each. This means the workflow waits for results one by one, in the order they were submitted, effectively merging them.
*   **Basic Aggregation:** The results from individual activities are combined to produce a final aggregated result.

**Docket features needed:**

*   **Parallel Step Execution:** The ability to define and execute multiple graph steps concurrently (fan-out).
*   **Asynchronous Result Handling:** Mechanisms (like futures or promises) to represent the eventual results of asynchronously executing graph steps.
*   **Ordered Join/Merge:** A primitive to collect results from parallel steps in a predefined order.
*   **Result Aggregation:** Support for combining results from multiple parallel steps into a single outcome.
*   **Basic Fan-out/Fan-in Pattern:** Fundamental building blocks for implementing distributed processing patterns where tasks can be divided and conquered.

## Split-Merge (Out-of-Order Completion with Selector)

**What it shows:** This example demonstrates the "split-merge" or "fan-out/fan-in" pattern, where multiple independent tasks (activities) are executed in parallel, and their results are collected and aggregated. Unlike the `splitmerge-future` example, this approach uses `workflow.Selector` to process the results *as soon as each activity completes*, regardless of the order in which they were initially started. This is crucial for optimizing throughput when the completion order of parallel tasks is not guaranteed or doesn't matter.
*   **Parallel Activity Execution:** Multiple `ChunkProcessingActivity` instances are launched concurrently.
*   **`workflow.Selector` for Out-of-Order Completion:** A `workflow.Selector` is configured to listen for the completion of any of the `workflow.Future` objects returned by the parallel activity executions.
*   **Event-Driven Result Processing:** As soon as a future becomes ready, its associated callback is executed, allowing for immediate processing of the result without waiting for other parallel tasks.
*   **Efficient Merging:** This pattern efficiently aggregates results from parallel tasks as they complete, maximizing throughput.

**Docket features needed:**

*   **Parallel Step Execution:** The ability to define and execute multiple graph steps concurrently (fan-out).
*   **Asynchronous Event Handling/Selection:** A mechanism (like a selector or event listener) to process results or events from multiple parallel branches as soon as they become available.
*   **Out-of-Order Join/Merge:** A primitive to collect results from parallel steps in any order, processing them as they complete.
*   **Result Aggregation:** Support for combining results from multiple parallel steps into a single outcome.
*   **Concurrency Control:** Tools for effectively managing and orchestrating parallel execution flows within the graph, including flexible waiting strategies.

## Workflow Start Delay

**What it shows:** This example demonstrates how to configure a Temporal workflow to start its execution after a specified delay. This is a fundamental feature for scheduling workflows to begin at a future point in time, without requiring the client to keep running or re-trigger the workflow.
*   **Delayed Workflow Execution:** The workflow execution is initiated by the client, but its actual processing by a worker is deferred until the `StartDelay` has elapsed.
*   **Simple Configuration:** The delay is configured directly in the `client.StartWorkflowOptions` using the `StartDelay` field.
*   **Client-Side Trigger, Server-Side Delay:** The client sends the request, but the Temporal server manages the delay, ensuring durability.
*   **Transparent to Workflow Logic:** The workflow code itself remains unaware of the initial delay, starting its execution normally once the delay expires.

**Docket features needed:**

*   **Delayed Graph Execution:** The ability to specify a delay before a graph execution begins processing its first step.
*   **Scheduling Options:** This is a basic form of scheduling, complementing more advanced cron-like scheduling.
*   **Durable Delays:** The delay mechanism should be durable, meaning the graph will still start after the specified time even if the client that initiated it goes offline.

## Synchronous Proxy

**What it shows:** This example demonstrates how to achieve synchronous, interactive communication with a long-running main workflow using a short-lived "proxy workflow." It addresses the common requirement for an external client (like a UI) to have a step-by-step conversation with a workflow, where each client request expects an immediate response before proceeding.
*   **Proxy Workflow Pattern:** A short-lived `UpdateOrderWorkflow` acts as an intermediary, receiving a request from the UI, forwarding it to the main `OrderWorkflow`, and then waiting for a response from the main workflow before returning to the UI.
*   **Signal-Based Synchronous Communication:** The proxy workflow and main workflow communicate using signals. The proxy sends a signal to the main workflow and then blocks (`proxy.ReceiveResponse`) waiting for a return signal from the main workflow.
*   **Interactive User Experience:** Enables building user interfaces that can have a natural, conversational flow with long-running backend processes.
*   **State Management in Main Workflow:** The main workflow (`OrderWorkflow`) manages the overall state of the order, validating inputs and orchestrating subsequent steps.
*   **Multi-Workflow Coordination:** Demonstrates how different workflows can interact and coordinate using signals.

**Docket features needed:**

*   **Synchronous Communication with Graphs:** A mechanism for external clients to initiate a synchronous request to a running graph instance and receive an immediate response.
*   **Inter-Graph Communication (Signals/Events):** Robust support for graphs to send and receive signals or events to/from other graphs for coordination.
*   **Blocking/Awaiting on External Events:** The ability for a graph step to pause execution and wait for a response or signal from another graph or external system.
*   **Proxy Graph Pattern:** Support for defining short-lived "proxy graphs" that can mediate synchronous interactions with long-running main graphs.
*   **Interactive Graph Interfaces:** Tools and patterns for building user interfaces that can interact with graphs in a step-by-step, conversational manner.

## Timers and Notifications

**What it shows:** This example demonstrates how to use Temporal's durable timers and selectors to manage delays and trigger conditional actions within a workflow. Specifically, it shows a scenario where a long-running activity is monitored by a timer, and a notification is sent if the activity exceeds a certain processing time, without necessarily canceling the long-running operation.
*   **Durable Timers (`workflow.NewTimer`):** Workflows can schedule timers that reliably fire after a specified duration, even across worker restarts or system outages.
*   **Event Handling with `workflow.Selector`:** A `workflow.Selector` is used to wait for the first of several events (activity completion or timer firing) to occur.
*   **Conditional Logic based on Timeouts:** The workflow executes different logic based on whether the activity completes before the timer or the timer fires first.
*   **Cancellation of Timers:** Timers can be canceled (`cancelHandler()`) if their purpose becomes redundant (e.g., the activity finishes before the timer expires).
*   **Non-Blocking Timeout/Notification:** The long-running activity continues to execute even if the timer fires, and a separate notification activity is triggered.

**Docket features needed:**

*   **Durable Timer Steps:** The ability to define graph steps that introduce reliable, long-duration delays.
*   **Conditional Event Waiting:** A mechanism to wait for the first of multiple events (e.g., step completion, timer expiry, external signals) to occur, and then proceed with specific logic.
*   **Cancellation of Pending Operations:** Support for canceling pending timer steps or other operations if their conditions are no longer relevant.
*   **Notification/Alerting Integrations:** The ability to trigger external notification activities or services based on time-based conditions or other events within the graph.
*   **Parallel Execution Control:** Managing concurrently executing steps and their interactions with timers and other events.

## Sleep For Days

**What it shows:** This example demonstrates a long-running workflow designed to execute a task periodically (e.g., once every 30 days) and to run indefinitely until it receives an explicit external signal to complete. This pattern is ideal for scheduled maintenance tasks, recurring reports, or any process that needs to repeat over long periods and be externally controllable.
*   **Long-Running Periodic Tasks:** Uses `workflow.NewTimer` to introduce long delays, effectively scheduling future executions of activities.
*   **External Control with Signals:** The workflow listens for a "complete" signal, allowing an external client to gracefully terminate its indefinite loop.
*   **`workflow.Selector` for Event Handling:** A `workflow.Selector` is used to concurrently wait for either the timer to fire or the completion signal to arrive, demonstrating robust event handling.
*   **Durable Scheduling:** The timer and signal mechanisms are fully durable, meaning the workflow will resume correctly even after worker restarts or system outages.

**Docket features needed:**

*   **Periodic Graph Execution:** Mechanisms to define graph steps that execute periodically (e.g., "every X days").
*   **External Event/Signal Handling:** The ability for graphs to receive and react to external signals or events for dynamic control (e.g., termination, modification).
*   **Delayed Execution (Timers):** Support for introducing long, durable delays within graph execution.
*   **Concurrent Event Waiting:** A primitive similar to `workflow.Selector` to wait for the first of several events (timers, signals) to occur.
*   **Indefinite Graph Lifecycles:** Support for graphs that are designed to run indefinitely until explicitly stopped or completed.

## Shopping Cart

**What it shows:** This example demonstrates a long-running, interactive shopping cart application using Temporal's "Update-with-Start" feature and updates. It showcases how to maintain state over extended periods and handle continuous modifications (adding/removing items) from an external client (a web UI).
*   **"Lazy-Init" with Update-with-Start:** The workflow is started (or reused if already existing) with `WorkflowIDConflictPolicy_USE_EXISTING`, allowing the client to continuously interact with the same workflow instance.
*   **Interactive Updates:** Uses `workflow.SetUpdateHandlerWithOptions` to define handlers that process requests (add, remove, list items) and provide immediate, synchronous responses to the UI.
*   **Internal State Management:** The workflow maintains the `CartState` (items and quantities) and updates it based on client requests.
*   **Signal for Checkout:** A "checkout" signal is used to trigger the finalization of the shopping cart.
*   **`continue-as-new` for History Bounding:** The workflow uses `continue-as-new` with an "idle period" check (`workflow.AllHandlersFinished(ctx)`) to manage its history size, ensuring it remains performant over many interactions.
*   **Input Validation:** The update handler includes a validator to ensure incoming requests are valid.

**Docket features needed:**

*   **Stateful Graph Execution:** Robust mechanisms for graphs to maintain and persist mutable state over long durations, suitable for interactive applications.
*   **Interactive Graph Update API:** A powerful API that allows external clients to send requests to a running graph instance and receive synchronous responses, similar to Temporal's Update API.
*   **Dynamic State Modification:** The ability for graph steps or handlers to modify the graph's internal state in response to external interactions.
*   **Signal/Event Handling:** Support for graphs to receive and react to external signals or events.
*   **"Lazy-Start" or "Upsert" Graph Execution:** A mechanism to start a graph if it doesn't exist, or interact with an existing one if it does, based on a unique identifier.
*   **Controlled History Management:** Advanced features for managing graph execution history, including equivalents to `continue-as-new`, ensuring that long-running interactive graphs do not accumulate excessive history.
*   **Input Validation for Interactions:** The ability to define validation rules for incoming requests/updates to a graph.

*   **Dynamic Metadata Update API:** An API for graphs to dynamically update their associated metadata during execution.
*   **Query Language for Graphs:** A query language or API that allows users to search and filter running and completed graph instances based on their metadata.
*   **Integration with Search/Indexing Services:** Underlying integration with search and indexing services to make metadata efficiently queryable.
*   **Visibility for Operational Monitoring:** Tools for operators to monitor and manage graph instances based on their custom metadata.

## Nexus Cancellation

**What it shows:** This example demonstrates how to cancel a Temporal Nexus operation from a caller workflow and how the handler workflow can gracefully respond to such cancellation requests. It specifically highlights the `NexusOperationCancellationTypeWaitRequested` cancellation type, where the caller waits only until the handler acknowledges the cancellation request, not until the handler fully completes its cancellation processing.
*   **Operation Cancellation:** Shows how a caller workflow can initiate the cancellation of a running Nexus operation.
*   **Cancellation Types:** Illustrates the use of `NexusOperationCancellationTypeWaitRequested`, which provides a balance between immediate caller notification and allowing the handler to perform some cleanup.
*   **Handler-Side Cancellation Handling:** The Nexus operation's backing workflow (`HelloHandlerWorkflow`) demonstrates how to detect `temporal.IsCanceledError` and perform cleanup or finalization logic within a `workflow.NewDisconnectedContext`.
*   **Controlled Shutdown:** Enables a controlled shutdown or cleanup of resources associated with a canceled operation.

**Docket features needed:**

*   **Cancelable Operations/Graphs:** A mechanism for external clients or other graphs to request the cancellation of a running graph execution or a specific operation within it.
*   **Cancellation Propagation:** Support for propagating cancellation signals effectively down to individual steps or sub-graphs.
*   **Graceful Cancellation Handling:** The ability for graph steps to detect cancellation requests and execute specific cleanup or compensation logic.
*   **Asynchronous Cancellation Acknowledgment:** For operations that might take time to clean up, a way for the graph to acknowledge cancellation quickly while continuing to finalize.
*   **Resource Deallocation on Cancellation:** Ensuring that resources held by a graph are properly released when it's canceled.

## Nexus Context Propagation

**What it shows:** This example extends the Temporal Nexus concept by demonstrating how custom context information can be seamlessly propagated across Nexus service boundaries. It builds upon the `ctxpropagation` example, showing how a custom `ContextPropagator` can be used in conjunction with Nexus to ensure that metadata (like a `caller-id`) travels from a calling workflow to a Nexus operation handler workflow, even if they reside in different namespaces or services.
*   **Cross-Service Context Flow:** Illustrates how custom data in the `context.Context` is preserved and passed from a Nexus client to a Nexus handler.
*   **`WorkerInterceptor` for Nexus Headers:** A specialized `WorkerInterceptor` is used to inject and extract custom context values into/from Nexus operation headers, transparently handling the serialization and deserialization.
*   **End-to-End Observability:** Enables advanced tracing and debugging by linking related operations across different services via propagated context.
*   **Custom Metadata Integration:** Provides a pattern for integrating custom, application-specific metadata into distributed calls.

**Docket features needed:**

*   **Cross-Graph Context Propagation:** Docket needs a mechanism to propagate custom context (similar to `context.Context` in Go) across graph boundaries, especially when one graph calls another.
*   **Custom Interceptors/Filters:** The ability to plug in custom logic that can inspect, modify, and enrich incoming and outgoing messages or requests, analogous to `WorkerInterceptor` for Nexus headers.
*   **Header-Based Metadata Transfer:** Support for transferring arbitrary key-value metadata in headers or other out-of-band mechanisms during inter-graph communication.
*   **Serialization for Context:** Mechanisms for serializing and deserializing context data for transmission.
*   **Security Considerations for Context Data:** Acknowledgment that context data in headers might be plain text and require encryption if sensitive.

## Nexus Multiple Arguments

**What it shows:** This example extends the Temporal Nexus concept by demonstrating how to map a Nexus operation (which inherently takes a single input argument) to a backing workflow that expects multiple, distinct arguments. This is achieved using `temporalnexus.NewWorkflowRunOperationWithOptions`, which provides more granular control over the mapping between Nexus operation input and workflow arguments.
*   **Flexible Argument Mapping:** Shows how a single Nexus operation input struct can be deconstructed into multiple arguments for the underlying workflow function.
*   **`NewWorkflowRunOperationWithOptions`:** Highlights the use of a more advanced constructor for defining Nexus operations, allowing for custom logic in mapping inputs.
*   **`ExecuteUntypedWorkflow`:** Demonstrates how to pass arguments to a workflow when its signature might not perfectly match a single input type.

**Docket features needed:**

*   **Flexible Argument Handling for Graph Operations:** When defining operations for Docket services, there needs to be a flexible mechanism to map incoming single input objects to multiple arguments for the underlying graph or subgraph execution.
*   **Advanced Operation Definition API:** Docket's API for defining operations needs to allow for custom mapping and transformation logic for inputs and outputs.
*   **Argument Deconstruction/Reconstruction:** Support for taking a structured input and breaking it down into individual parameters for steps, and vice-versa for collecting results.
*   **Type Marshaling/Unmarshaling:** Robust mechanisms for converting between structured data (like JSON or Protobuf) and the native argument types expected by graph steps.

## Particle Swarm Optimization (PSO)

**What it shows:** This is a highly complex and illustrative example demonstrating a long-running, iterative, and compute-intensive process (Particle Swarm Optimization) leveraging several advanced Temporal features.
*   **Long-Running Iterative Processes:** Models a multi-step, iterative optimization algorithm.
*   **Parent-Child Workflows:** A parent workflow orchestrates multiple attempts of the optimization, each executed by a child workflow.
*   **`continue-as-new` for History Bounding:** The child workflow uses `workflow.NewContinueAsNewError` to restart itself periodically. This is crucial for preventing workflow history from growing indefinitely during long-running iterative computations, ensuring efficient recovery and replay.
*   **Parallel Computation:** Within the child workflow, individual particles (computation units) are likely processed in parallel using `workflow.Go` (as hinted by the README) for concurrent execution.
*   **Custom `DataConverter`:** A custom `DataConverter` is used to efficiently serialize and deserialize complex data structures representing the optimization state, crucial for passing data between workflow iterations and activities.
*   **Query API for Live State:** The workflow exposes its current state via the Query API, allowing external systems to monitor the progress of the optimization.
*   **Deterministic Randomness:** Uses `workflow.SideEffect` for generating random elements deterministically within the workflow.

**Docket features needed:**

*   **Long-Running Iterative Graph Execution:** Support for defining and executing graphs that involve many iterations, potentially spanning long periods.
*   **`Continue-As-New` Equivalent for Graphs:** A mechanism for a graph to effectively "restart" itself with updated state, preserving a bounded execution history for iterative processes.
*   **Hierarchical Graph Structures (Parent-Child):** The ability to define and manage parent-child relationships between graphs, where a parent graph orchestrates child graph executions.
*   **Parallel Processing within Graphs:** Robust support for concurrent execution of graph steps or sub-graphs, enabling parallel computations.
*   **Customizable Data Serialization/Deserialization:** A pluggable mechanism for handling the serialization and deserialization of complex custom data types used within graph steps and between graph iterations.
*   **Live State Querying:** An API to query the real-time state and progress of a running graph execution.
*   **Deterministic Execution Environment:** Features to ensure determinism for operations that might otherwise be non-deterministic (e.g., random number generation) within the graph.

## Workflow Recovery

**What it shows:** This advanced example demonstrates a robust pattern for recovering from problematic workflow executions (e.g., stuck due to bugs, corrupted state, or intentional restarts). It involves a dedicated "Recovery Workflow" that can:
*   **Identify Problematic Workflows:** Query the Temporal service to list all outstanding (open) workflow executions of a specific type.
*   **Parallel Recovery Execution:** Concurrently processes multiple identified workflows for recovery using parallel activities.
*   **Fetch History:** Retrieves the complete event history of a workflow execution.
*   **Extract State and Signals:** Parses the workflow history to extract the original start parameters and all signals sent to the workflow.
*   **Terminate Existing Runs:** Gracefully terminates currently running instances of the problematic workflows.
*   **Start New Runs:** Initiates new workflow executions with the original start parameters.
*   **Replay Signals:** Replays all historical signals to the newly started workflow run, effectively resuming the workflow from its last known good state or re-applying all past events.
*   **Activity Heartbeating for Progress:** Recovery activities use heartbeats to report progress and enable resuming from the last processed item if interrupted.

**Docket features needed:**

*   **Graph Management API:** A comprehensive API for managing running graph instances, including listing, querying, terminating, and restarting.
*   **History Access and Replay:** The ability to access the full execution history of a graph and to replay it against a potentially modified graph definition or new execution.
*   **State Extraction and Injection:** Mechanisms to extract the starting state (initial parameters, signals) from a graph's history and inject it into a new graph execution.
*   **Inter-Graph Operational Control:** The ability for one graph (e.g., a recovery graph) to exert operational control over other graphs (e.g., stopping them, restarting them, sending them signals).
*   **Batch Processing for Management Tasks:** Support for parallelizing management tasks (like recovery of multiple graphs) across available workers.
*   **Resumable Management Activities:** Management activities should be resumable and fault-tolerant, potentially leveraging heartbeating for progress tracking.

## Request/Response with Activity-Based Responses

**What it shows:** This example demonstrates a sophisticated request/response pattern where workflows accept requests via signals and send responses back to the requester via a dedicated "response activity". This pattern is highly efficient for push-based communication from the workflow back to the client and provides mechanisms for managing long-running state.
*   **Signal for Request, Activity for Response:** Requests are sent as signals to the workflow, and the workflow then executes an activity (potentially on the requester's worker) to deliver the response.
*   **Long-Running Workflow with `continue-as-new`:** The workflow uses `continue-as-new` to keep its history bounded, preventing it from growing indefinitely when processing many requests.
*   **Idle Period for `continue-as-new`:** It demonstrates the critical consideration of ensuring an "idle period" (no pending signals or activities) before a `continue-as-new` is executed to avoid data loss.
*   **Dynamic Response Activity:** The response activity's task queue is dynamically specified in the request, allowing for flexible routing of responses.
*   **Workflow-Internal State Management:** The `uppercaser` struct manages internal state for processing requests, showing how to organize business logic within the workflow.

**Docket features needed:**

*   **Request/Response Primitives:** Docket needs built-in support for request/response interactions with graphs, including mechanisms for sending requests (e.g., signals) and receiving responses (e.g., via dedicated "response steps" or callbacks).
*   **Asynchronous Communication:** The ability for graphs to communicate asynchronously, with support for push-based responses.
*   **External Interaction with Graph:** APIs for external systems to interact with running graph instances (send requests, receive responses).
*   **`Continue-As-New` Equivalent for Graph Instances:** A mechanism for long-running graph instances to `continue-as-new` to manage history size.
*   **Conditional Execution/Waiting:** The ability to define conditional waiting within a graph (e.g., waiting for all outstanding response steps to complete before continuing).
*   **Dynamic Step Dispatch:** The ability to dynamically select and execute different response steps based on request parameters.

## Request/Response with Query-Based Responses

**What it shows:** This example provides an alternative approach to the request/response pattern, utilizing workflow queries for delivering responses. The workflow receives requests via signals (similar to `reqrespactivity`), processes them, and then stores the result internally. Clients then actively poll the workflow using a query to retrieve the processed result.
*   **Signal for Request, Query for Response:** Requests are sent as signals, but responses are retrieved by the client querying the workflow's state.
*   **No Requester Worker:** This pattern eliminates the need for a dedicated worker on the client side to receive activity callbacks, simplifying client deployments.
*   **History Optimization:** Query results are not recorded in the workflow's event history, which can be beneficial for keeping history size manageable for long-running workflows with frequent interactions.
*   **Workflow-Managed State:** The workflow explicitly stores the results in its internal state (`responses` map) for clients to query.
*   **"Idle Period" for `continue-as-new`:** Similar to the activity-based response example, this workflow also uses `continue-as-new` and carefully manages the idle period to ensure proper history management.

**Docket features needed:**

*   **Request/Response Primitives (Query-based):** Support for accepting requests and making responses available via a query mechanism, where the client actively polls for results.
*   **Internal State Management:** The graph engine needs to provide robust mechanisms for managing internal state that can be exposed via queries.
*   **Query API for Graph State:** An API for external clients to query the current state or results of a running graph execution.
*   **Signal Handling for Requests:** The ability for graphs to receive and process external signals as requests.
*   **History Optimization for Long-Running Graphs:** Mechanisms like `continue-as-new` equivalents to manage the execution history of continuously running graph instances.

## Workflow Scheduling

**What it shows:** This example demonstrates how to create and manage schedules for Temporal workflows. It showcases the full lifecycle of a schedule, from creation and manual triggering to updating its specification, pausing, unpausing, and deleting. Scheduled workflows are a powerful alternative to cron jobs, offering enhanced reliability, visibility, and management capabilities.
*   **Schedule Creation:** How to define a new schedule with a workflow action and an optional initial specification.
*   **Manual Triggering:** The ability to trigger a scheduled workflow immediately, outside of its defined schedule.
*   **Dynamic Schedule Updates:** Modifying a schedule's recurrence patterns (e.g., cron-like expressions, intervals) and other properties (e.g., `RemainingActions`, `Paused` state) at runtime.
*   **Schedule Lifecycle Management:** Controlling the operational state of a schedule (pausing, unpausing, deleting).
*   **Schedule Metadata Access:** The scheduled workflow can access information about its own scheduled invocation (e.g., `TemporalScheduledById`, `TemporalScheduledStartTime`) via search attributes.

**Docket features needed:**

*   **Graph Scheduling:** A mechanism to define and manage recurring executions of graphs based on cron expressions or other scheduling parameters.
*   **Schedule Management API:** A comprehensive API to create, read, update, delete, trigger, pause, and unpause graph schedules.
*   **Scheduled Execution Metadata:** The ability for scheduled graph executions to access metadata about their invocation (e.g., scheduled start time, trigger ID).
*   **Integration with External Schedulers:** If Docket doesn't provide native scheduling, it should have clear integration points with external scheduling services.
*   **Event-Driven Scheduling:** The possibility of scheduling graphs based on external events, not just time-based triggers.

## Activity Retries

**What it shows:** This example demonstrates how to configure and handle automatic retries for activities within a Temporal workflow. It's a fundamental pattern for building fault-tolerant systems, allowing activities to recover from transient failures without requiring explicit error handling logic in the workflow.
*   **Configurable Retry Policy:** The workflow defines a `RetryPolicy` for activities, specifying parameters like `InitialInterval`, `BackoffCoefficient`, `MaximumInterval`, and `MaximumAttempts`.
*   **Activity Heartbeating for Progress:** The activity periodically records heartbeats (`activity.RecordHeartbeat`). This allows Temporal to detect if an activity is still alive and making progress, and to record the last known progress.
*   **Resuming from Progress on Retry:** If an activity fails and is retried, it can check `activity.HasHeartbeatDetails` and `activity.GetHeartbeatDetails` to resume processing from the last reported progress point, making the activity idempotent and efficient for long-running tasks.
*   **Simulated Failures:** The example intentionally introduces failures in the activity to demonstrate how the retry policy and heartbeat mechanism work.

**Docket features needed:**

*   **Step Retry Configuration:** The ability to define and configure retry policies for individual graph steps, including various parameters like intervals, backoff, and maximum attempts.
*   **Resumable Step Execution:** Support for graph steps to report progress and resume execution from the last successful point after a retry, crucial for long-running, fault-tolerant operations.
*   **Heartbeating/Liveness Checks:** A mechanism for graph steps to periodically signal their liveness and progress to the Docket engine.
*   **Error Handling and Recovery:** Robust error handling capabilities within the graph, including automatic retry logic for transient failures.
*   **Context for Compensation:** Providing necessary context and data to compensation steps to allow them to correctly reverse the effects of prior actions.

## Server-Side JWT Authentication

**What it shows:** This example demonstrates how to secure communication with the Temporal server using JWT (JSON Web Token) for authorization. It involves configuring the Temporal server to validate JWTs and configuring clients (starters and workers) to generate and include JWTs in their requests. This is crucial for multi-tenant environments and for enforcing fine-grained access control to Temporal resources.
*   **JWT-Based Authentication:** Clients generate and sign JWTs with specific claims (e.g., `permissions`) for authentication and authorization.
*   **Server-Side Validation (JWKS):** The Temporal server validates incoming JWTs against public keys exposed through a JWKS (JSON Web Key Set) endpoint.
*   **Permissions-Based Authorization:** The server uses the `permissions` claim within the JWT to determine what operations the client is authorized to perform (e.g., read/write on a specific namespace).
*   **Transparent to Workflow/Activity Logic:** The core workflow and activity code remains unaware of the underlying JWT authentication mechanism.
*   **Secure Client-Server Communication:** Ensures that all interactions are authenticated and authorized, enhancing the overall security posture.

**Docket features needed:**

*   **Pluggable Authentication/Authorization:** Docket's client and server components (if applicable) need to support a pluggable system for various authentication and authorization mechanisms, including JWTs.
*   **Credential Management:** Secure handling and injection of cryptographic keys (private keys for signing JWTs, public keys for verification).
*   **Policy Enforcement:** The ability to define and enforce access control policies based on user roles, permissions, or other attributes derived from authentication tokens.
*   **Secure Communication Channels:** Underlying support for secure transport protocols (like TLS) to protect the integrity and confidentiality of communication.

## Saga

**What it shows:** This example demonstrates the "Saga pattern" for managing distributed transactions and ensuring data consistency across multiple services. A Saga is a sequence of local transactions, where each transaction updates data within a single service. If a local transaction fails, the Saga executes compensating transactions to undo the changes made by preceding successful transactions.
*   **Distributed Transaction Management:** Provides a robust solution for ensuring atomicity and consistency in complex distributed operations.
*   **Compensating Transactions:** Uses dedicated "compensation activities" (`WithdrawCompensation`, `DepositCompensation`) to reverse the effects of successful prior steps if a later step fails.
*   **Sequential Execution with Error Handling:** Activities are executed in a defined order, with `defer` blocks strategically placed to trigger compensation logic upon failure.
*   **Fault Tolerance:** The workflow is designed to gracefully handle failures at any step by executing the necessary compensation.

**Docket features needed:**

*   **Compensating Step/Graph Support:** The ability to define and execute compensating steps or sub-graphs that are automatically triggered upon failure of a preceding step or branch.
*   **Declarative Compensation Logic:** A way to declaratively specify the compensation actions associated with each graph step.
*   **Atomic Rollback:** Mechanisms to ensure that compensation actions effectively roll back the state to a consistent point.
*   **Advanced Error Handling for Branches:** More sophisticated error handling capabilities than simple retries, specifically for distributed transaction patterns.
*   **Context for Compensation:** Providing necessary context and data to compensation steps to allow them to correctly reverse the effects of prior actions.

## Safe Message Handler (Cluster Manager)

**What it shows:** This advanced example demonstrates how to build robust, long-running workflows with atomic message handling, shared state protection, and efficient history management using `continue-as-new`. It models a cluster manager that assigns and deletes jobs to nodes, handling requests via signals and updates.
*   **Atomic Updates:** Uses `workflow.Mutex` to protect shared workflow state (e.g., node assignments) from race conditions when multiple update requests arrive concurrently.
*   **State-Dependent Processing:** Signal and update handlers only process messages when the workflow is in an appropriate state (`ClusterStarted`), enforced by `workflow.Await`.
*   **Long-Lived State Management with `continue-as-new`:** Periodically restarts itself using `continue-as-new` to maintain a bounded history. The workflow carefully waits for all pending message handlers (signals and updates) to complete before restarting (`workflow.AllHandlersFinished(ctx)`), preventing data loss.
*   **Request/Response via Updates and Signals:** Demonstrates a blend of synchronous updates (for node assignments) and asynchronous signals (for cluster start/shutdown) to interact with the workflow.
*   **Idempotent Message Handling:** The update handlers are designed to be idempotent, ensuring that processing a message multiple times has the same effect as processing it once.

**Docket features needed:**

*   **Distributed Mutex/Lock Primitive:** A mechanism to protect shared graph state or resources across concurrent graph executions or interactive requests.
*   **Stateful Graph Execution:** Robust support for managing and persisting complex, mutable state across long-running graph executions and `continue-as-new` cycles.
*   **Atomic State Changes:** Guarantees for atomic updates to graph state, especially in response to concurrent external interactions.
*   **Conditional Event Processing:** The ability for graph handlers (for signals, updates, queries) to execute based on specific conditions of the graph's internal state.
*   **Message Buffering and Processing Guarantees:** Ensuring reliable processing of incoming messages (signals, updates) even during transitions like `continue-as-new`.
*   **Advanced `Continue-As-New` Management:** Sophisticated mechanisms for orchestrating `continue-as-new` in stateful, interactive graphs, including waiting for all pending operations to complete.
*   **Idempotent Step Definitions:** Encouraging or facilitating the definition of idempotent graph steps and handlers for increased robustness.


## Request/Response with Update-Based Responses

**What it shows:** This example showcases the recommended and most efficient pattern for implementing interactive request/response in Temporal: using workflow "Updates". Updates provide a synchronous, request-response mechanism where the client sends an update and receives an immediate response from the workflow, significantly improving responsiveness and simplifying interaction compared to signals or queries.
*   **Synchronous Request-Response:** Clients send an update to a running workflow and immediately receive a response.
*   **Direct Workflow Interaction:** Updates interact directly with the workflow logic, allowing for immediate state changes and responses.
*   **Validator for Updates:** The update handler can include a validator to reject invalid updates proactively, before they are even recorded in the workflow history. This is crucial for maintaining workflow integrity and managing history size.
*   **`continue-as-new` with Update Management:** The workflow carefully manages `continue-as-new` by ensuring all in-flight update requests are processed and no pending updates exist before restarting. This prevents history bloat in long-running, interactive workflows.
*   **"Superior" Approach:** This pattern is presented as generally superior to signal-based or query-based request/response for most interactive use cases.

**Docket features needed:**

*   **Synchronous Interaction API:** A primary API for external clients to send synchronous requests to a running graph execution and receive immediate responses. This would be analogous to Temporal's Update API.
*   **In-Graph Request Handling:** The ability for graph instances to define and register handlers for incoming requests, process them, and generate responses.
*   **Pre-execution Validation:** A mechanism to validate incoming requests or updates before they are processed by the graph logic, potentially rejecting them early.
*   **Stateful Interaction Management:** Support for managing the internal state necessary to process interactive requests and generate responses.
*   **Integrated History Management:** Advanced history management features for long-running, interactive graphs that involve many updates, potentially using `continue-as-new` equivalents while ensuring request integrity.
*   **Concurrency Control for Interactive Requests:** Mechanisms to handle multiple concurrent interactive requests to a single graph instance.

## Polling (Frequent Activity Polling)

**What it shows:** This example demonstrates how to implement frequent polling within a Temporal activity. It's designed for scenarios where an external service needs to be checked rapidly (e.g., every second) until a condition is met. The key to its robustness lies in actively heartbeating from the activity.
*   **Active Heartbeating:** The activity explicitly calls `activity.RecordHeartbeat(ctx)` on each polling iteration. This signals to the Temporal server that the activity is still alive and making progress, preventing it from timing out and being re-scheduled unnecessarily.
*   **Short `HeartbeatTimeout`:** Coupled with a long `StartToCloseTimeout`, a short `HeartbeatTimeout` ensures that if the worker processing the activity fails, Temporal quickly detects the failure and restarts the activity on another worker.
*   **Loop-based Polling:** The polling logic resides within a `for` loop inside the activity, continuously checking the external service.
*   **Graceful Cancellation:** The activity uses `ctx.Done()` to detect if the workflow (and thus the activity) has been canceled, allowing it to exit cleanly.

**Docket features needed:**

*   **Looping/Retry for Step Execution:** The ability to define steps that continuously retry or poll an external service until a condition is met or a timeout occurs.
*   **Heartbeating/Progress Reporting:** A mechanism for long-running graph steps to report their progress and indicate liveness, allowing the Docket engine to detect and recover from failures efficiently.
*   **Configurable Timeouts:** Fine-grained control over various timeouts (e.g., `StartToCloseTimeout`, `HeartbeatTimeout`) for individual graph steps.
*   **Cancellation Handling within Steps:** Support for steps to detect and gracefully respond to cancellation requests.
*   **External Service Integration:** Native or extensible capabilities for polling external APIs or services.

## OpenTelemetry Integration

**What it shows:** This example demonstrates how to integrate Temporal workflows with OpenTelemetry for distributed tracing. It illustrates how to instrument a Temporal application (starter, worker, workflow, and activities) to generate traces that can be exported to various OpenTelemetry-compatible tracing backends (e.g., Jaeger, Honeycomb.io).
*   **Automatic Instrumentation:** Temporal's SDK provides `TracingInterceptor` to automatically create and propagate spans for workflow and activity executions.
*   **Manual Instrumentation:** Shows how to perform explicit OpenTelemetry calls within workflow and activity code (e.g., `tracer.Start`, `span.SetAttributes`, `span.AddEvent`) for more detailed tracing.
*   **Trace Context Propagation:** Ensures that trace context (trace ID, span ID) is propagated across all components of a distributed Temporal application.
*   **Standardized Observability:** Leverages OpenTelemetry, a vendor-neutral standard for instrumentation, enabling flexible backend choices.

**Docket features needed:**

*   **Distributed Tracing Integration (OpenTelemetry compatible):** Docket needs first-class support for distributed tracing, preferably via OpenTelemetry, to provide end-to-end visibility of graph executions across services.
*   **Automatic Span Generation:** The ability to automatically generate spans for graph executions, steps, and sub-graphs, capturing their duration, status, and metadata.
*   **Context Propagation for Tracing:** Mechanisms to propagate trace context across graph boundaries and to external services invoked by graph steps.
*   **Manual Instrumentation API (Optional):** An API for users to add custom spans, attributes, and events within their graph step implementations for fine-grained control over tracing.
*   **Configurable Trace Exporters:** The ability to configure various trace exporters (e.g., OTLP, Jaeger, Zipkin) to send trace data to different backends.

## Pick First

**What it shows:** This example demonstrates the "pick first" pattern, where a workflow initiates multiple parallel activities and then proceeds with the result of the activity that completes first. Once the first result is obtained, the workflow cancels the remaining in-flight activities. This pattern is highly useful for optimizing latency by parallelizing alternative approaches or redundant computations.
*   **Parallel Execution with `workflow.Selector`:** Multiple activities are started concurrently, and a `workflow.Selector` is used to wait for the first one to complete.
*   **Cancellation of Redundant Work:** Once a result is obtained, a cancelable context is used to send cancellation requests to the other pending activities, stopping unnecessary work.
*   **Activity Heartbeating and Cancellation Handling:** Activities periodically send heartbeats and check their context (`ctx.Done()`) to gracefully handle cancellation requests.
*   **Optimizing for Latency:** A core use case is to try multiple ways to achieve a goal and use the fastest one, canceling the slower ones.

**Docket features needed:**

*   **Race Condition/Selector Primitive:** A mechanism for a graph to await the completion of the first of several parallel branches or steps, and then potentially react to that completion.
*   **Conditional Cancellation:** The ability to cancel specific branches or steps of a graph based on the outcome of other parallel branches.
*   **Activity/Step Heartbeating:** For long-running steps, a mechanism for them to periodically report progress, which can also serve as a hook for detecting cancellation.
*   **Graceful Step Shutdown:** Graph steps need a way to detect and respond to cancellation signals, allowing them to clean up resources.
*   **Concurrency Management:** Support for managing and orchestrating parallel execution flows within the graph, including mechanisms for starting, waiting for, and canceling concurrent tasks.

## Polling (Infrequent Activity Polling)

**What it shows:** This example demonstrates an efficient pattern for infrequent polling of external services, typically for intervals of a minute or longer. Instead of implementing polling logic within the activity or using workflow timers, this approach leverages Temporal's activity retry mechanism.
*   **Activity Retry-Based Polling:** The activity's `RetryPolicy` is configured with `InitialInterval` set to the desired poll frequency and `BackoffCoefficient` to 1. This ensures that if the activity fails (e.g., the external service is not ready), it will be retried at fixed, regular intervals.
*   **History Optimization:** A major advantage is that individual activity retries are not recorded in the workflow's history, preventing history bloat for long-running polling operations. Only the final success or failure is recorded.
*   **Decoupled Polling Logic:** The activity itself remains simple, focusing solely on making the external call and returning its result. The polling and retry logic is externalized to the Temporal platform.
*   **Fault-Tolerant Polling:** The Temporal platform automatically manages retries, handling worker crashes or network issues seamlessly.

**Docket features needed:**

*   **Step Retry Policies:** Docket needs a robust mechanism to define retry policies for graph steps, including configurable initial intervals and backoff coefficients, to support polling patterns.
*   **External Service Integration (Implicit Retries):** The ability to integrate with external services where Docket handles the retry logic for polling activities transparently.
*   **History/Audit Log Optimization:** Features to ensure that repetitive polling attempts or retries do not excessively bloat the graph's execution history or audit logs.
*   **Configurable Step Timeouts:** The ability to set short `StartToCloseTimeout` for polling steps that are expected to either succeed or fail quickly.

## Polling (Periodic Sequence via Child Workflow)

**What it shows:** This example demonstrates a sophisticated pattern for periodic polling, especially useful when the polling operation involves a sequence of activities or requires dynamic changes to arguments between attempts. It uses a child workflow that executes the polling logic in a loop and then calls `continue-as-new` to restart itself, thereby maintaining a bounded workflow history for long-running periodic tasks.
*   **Child Workflow for Polling:** A dedicated child workflow encapsulates the polling logic, allowing for modularity and isolation.
*   **Sequential Activity Execution within Poll Cycle:** Each polling iteration can involve a sequence of activities.
*   **`workflow.Sleep` for Intervals:** `workflow.Sleep` is used to introduce delays between polling attempts, establishing the periodic nature.
*   **`continue-as-new` for History Management:** The child workflow periodically calls `workflow.NewContinueAsNewError` to restart itself. This is crucial for long-running polling scenarios as it prevents the workflow's history from growing indefinitely.
*   **Dynamic Polling Arguments:** Arguments for activities within the polling sequence can be modified before each retry or `continue-as-new`.
*   **Parent-Child Decoupling:** The parent workflow remains largely unaware of the `continue-as-new` cycles within the child, simply waiting for its eventual completion.

**Docket features needed:**

*   **Child Graph Execution for Sub-Processes:** Docket needs to support the execution of child graphs for encapsulating sub-processes or periodic tasks.
*   **Looping Constructs within Graphs:** The ability to define loops within a graph for repeating a sequence of steps.
*   **Delayed Execution (Timers):** A mechanism to introduce pauses or delays (`workflow.Sleep`) within graph execution.
*   **`Continue-As-New` Equivalent for Graphs:** A feature to allow a long-running graph to restart itself with updated state while maintaining a bounded history.
*   **Dynamic Argument Passing between Graph Iterations:** The ability to modify arguments for steps in subsequent iterations of a loop or `continue-as-new` cycle.
*   **Robust Fault Tolerance for Loops:** Ensuring that loops within a graph are fault-tolerant and can resume correctly after failures.

## Workflow Goroutines

**What it shows:** This example demonstrates how to achieve concurrency and parallelism within a Temporal workflow using `workflow.Go`. Crucially, it highlights that native Go goroutines (`go func()`) should *not* be used directly in workflows due to their non-deterministic nature. Instead, `workflow.Go` provides a deterministic alternative that allows for spawning concurrent execution paths that are replayable and fault-tolerant.
*   **Deterministic Concurrency:** `workflow.Go` enables parallel execution of logical units (like sequences of activities) while preserving the deterministic nature required for Temporal's fault-tolerance.
*   **Parallel Activity Sequences:** Shows how to initiate multiple independent sequences of activities concurrently.
*   **Synchronization:** Uses `workflow.Await` to wait for the completion of all spawned "goroutines," effectively acting as a join point for parallel branches.
*   **Context Management:** Emphasizes passing a derived context (`gCtx`) to the `workflow.Go` function to ensure proper isolation and determinism.

**Docket features needed:**

*   **Deterministic Parallelism Primitive:** A core mechanism in Docket to define and execute multiple branches of a graph concurrently in a deterministic fashion. This would be analogous to `workflow.Go`.
*   **Synchronization Points:** The ability to define points in the graph where execution must wait for all preceding parallel branches to complete (join).
*   **Controlled Concurrency:** Providing means to manage the number of concurrent branches or steps, similar to the `parallelism` input in this example.
*   **Context Scoping:** Proper management of execution contexts for concurrent branches, ensuring data integrity and deterministic behavior across replays.

## Eager Workflow Start

**What it shows:** This example demonstrates "Eager Workflow Start" (EWS), a performance optimization feature in Temporal. EWS aims to reduce workflow latency by allowing the starter and worker to interact directly, bypassing the server for initial task processing. This is particularly effective when the starter and worker are collocated (e.g., running in the same process or on the same machine). The example shows how to configure a client to request eager execution and how a worker can handle it.

**Docket features needed:**

*   **Performance Optimizations:** While not directly a workflow logic feature, Docket should consider and allow for various performance optimizations, potentially including eager execution models. This might involve:
    *   **Direct Execution Paths:** The ability for the graph executor to detect collocated steps (e.g., adjacent activities) and execute them directly without intermediate queuing if possible.
    *   **Optimized Task Scheduling:** Mechanisms to prioritize and schedule tasks efficiently, especially for latency-sensitive operations.
*   **Configuration for Execution Modes:** The ability to configure different execution modes or policies (e.g., eager vs. standard) at the graph or step level, allowing users to choose performance characteristics.
*   **Local Execution Capabilities:** Support for executing graph steps locally within the same process as the orchestrator when network hops are undesirable.

## Early Return (Update-with-Start)

**What it shows:** This advanced example demonstrates a pattern called "Early Return" using Temporal's "Update-with-Start" feature. It allows a client to initiate a workflow execution and immediately receive an intermediate result from the workflow via an update, while the workflow continues its longer-running tasks in the background. This is useful for improving perceived responsiveness in applications where a partial result is sufficient for immediate feedback.
*   **Update-with-Start:** Combines starting a workflow with sending an initial update, enabling synchronous retrieval of an intermediate result.
*   **Responsive Initialization:** The workflow's initial phase performs a quick operation (e.g., validation, resource allocation) and returns its result via an update, allowing the client to proceed without waiting for the entire workflow to finish.
*   **Asynchronous Continuation:** The remainder of the workflow's logic (e.g., complex processing, cleanup) continues asynchronously after the early return.
*   **`workflow.SetUpdateHandler`:** Used within the workflow to define the logic that processes the incoming update and returns the intermediate result.
*   **`workflow.Await` in Update Handler:** Ensures the update handler waits for the necessary initial work to complete before returning.

**Docket features needed:**

*   **Synchronous Intermediate Results:** A mechanism for a graph execution to return an intermediate result to its caller while continuing its own execution asynchronously. This would involve a dedicated API similar to "Update-with-Start".
*   **Graph Update/Interaction API:** The ability for external clients to interact with a running graph execution, not just to start it or query its final state, but to send updates and receive immediate responses.
*   **Conditional Execution within Update Handlers:** Support for conditional waiting (`workflow.Await`) within the logic that processes these intermediate interactions.
*   **Clear Separation of Concerns:** Docket should facilitate the definition of initial, quick-return phases and subsequent long-running asynchronous phases within a single graph.
*   **Configurable Server Features:** Awareness of and ability to configure underlying platform features (like `frontend.enableExecuteMultiOperation`) that enable such advanced interaction patterns.
