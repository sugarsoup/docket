# SciPipe vs. Docket Comparison

## Overview

| Feature | SciPipe | Docket |
| :--- | :--- | :--- |
| **Core Philosophy** | **Scientific Flow-Based Programming (FBP)**: Treating a workflow as a network of independent processes (shell commands) that pass files via channels. | **Type-Safe Code Execution**: Treating a graph as a series of Go function calls passing struct/proto data. |
| **Definition** | **Imperative Go Code**: You explicitly create `scipipe.NewProc()`, connect `proc1.Out("foo").To(proc2.In("bar"))`. | **Declarative Graph Registry**: You register functions (`Register(MyFunc)`), and the engine auto-wires dependencies based on type signatures. |
| **Data Model** | **Files & Streams**: The primary unit of exchange is a File path. Designed for Linux pipes (`|`) and heavy I/O. | **Protobuf Messages**: The primary unit is a typed message. Designed for business logic, API orchestration, and metadata. |
| **Execution Model** | **Multi-Process / Local**: Runs a swarm of local processes/goroutines that spawn shell commands. Excellent for "batch jobs on a big server". | **Single Process / Distributed**: Designed to run within a Go application. Can be distributed (River/Temporal), but fundamentally executes code, not shell scripts. |
| **Audit/Reproducibility** | **Audit Logs (JSON)**: Generates detailed logs of every command, flag, and file hash executed for scientific reproducibility. | **Execution History (Event Sourcing)**: (Planned) Tracks logical steps, inputs, and outputs for debugging and retries. |

## Key Learnings from SciPipe for Docket

SciPipe is excellent at "gluing together black boxes" (shell commands). Docket is about "orchestrating white boxes" (Go functions). However, SciPipe's focus on **data lineage** and **file management** offers valuable lessons.

### 1. The "Audit Trail" (Data Lineage)
*   **SciPipe:** Produces an `audit.json` for every output file, detailing exactly *how* it was created (command, upstream files, execution time).
*   **Docket Today:** Tracks execution, but mostly for retries/debugging.
*   **Lesson:** Business logic often needs "Explainability" (Why did this fraud check fail?).
    *   *Action:* Add a standard "Trace/Audit" metadata field to the execution context.
    *   *Feature:* When a step runs, it can attach "Evidence" (e.g., "Used Policy version v1.2") to the execution log, creating a queryable lineage for the final result.

### 2. Flexible File Naming & Path Management
*   **SciPipe:** Allows complex formatting of output filenames (`{i:ref_genome}.{p:window_size}.bam`).
*   **Docket Today:** Doesn't handle files explicitly.
*   **Lesson:** If Docket handles blobs (as per the Reflow analysis), it needs a way to *name* them logically so humans can find them in S3.
    *   *Feature:* A standard way to derive "Asset Keys" (S3 paths) from Step Inputs. E.g., `s3://bucket/reports/{date}/{user_id}.pdf`.

### 3. Streaming Data (Pipes)
*   **SciPipe:** Can stream data through Unix pipes between processes, avoiding disk I/O for intermediate steps.
*   **Docket Today:** Passes full objects in memory.
*   **Lesson:** For large collections (e.g., "Process 1M User Records"), loading all into RAM is bad.
    *   *Feature:* **Iterators / Streams**. Instead of `[]*User`, a step could return `<-chan *User`. The Executor reads from the channel and feeds the next step item-by-item (pipeline parallelism). This effectively replicates the "Unix Pipe" model but with typed objects.

## Roadmap Additions

Based on SciPipe, we should prioritize:

1.  **P2: Streaming Support:** Support `chan T` or `Iterator<T>` as valid step inputs/outputs to handle large datasets without OOM.
2.  **P3: Audit/Lineage API:** A formalized way for steps to record "decisions" or "evidence" separate from their direct output, for compliance/auditing.
