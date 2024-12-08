  The Core Concept: "Snapshot & Resume"

  Unlike Temporal, which replays history to rebuild state, Docket already stores state in the executor.cache (a map of reflect.Type -> proto.Message).

  To persist, we simply need to:
   1. Snapshot: Serialize the executor.cache (the results computed so far) and the list of completed_steps.
   2. Enqueue: Create a River Job containing this snapshot.
   3. Resume: A worker picks up the job, deserializes the cache, and calls Execute again. The executor sees the results are already in the cache and skips those
      steps, proceeding immediately to the next unfinished step.

  ---

  1. The Data Structure (The Job Args)

  We define a generic River Job that represents a "pending graph execution".

    1 package persistence
    2 
    3 import "riverqueue/river"
    4 
    5 // GraphExecutionArgs is the River job payload.
    6 type GraphExecutionArgs struct {
    7     // Unique ID to prevent duplicate executions
    8     ExecutionID string `json:"execution_id"`
    9     
   10     // The name of the graph/workflow to run (needs a registry)
   11     GraphName string `json:"graph_name"`
   12     
   13     // The "Snapshot": Serialized Protobuf results of steps completed so far.
   14     // Key: Proto Message Full Name, Value: Bytes
   15     CacheSnapshot map[string][]byte `json:"cache_snapshot"`
   16     
   17     // Initial inputs (serialized)
   18     Inputs map[string][]byte `json:"inputs"`
   19 }
   20 
   21 func (GraphExecutionArgs) Kind() string { return "protograph_execution" }

  2. Scenario Implementation

  Here is how we handle the three specific scenarios you requested.

  Scenario A: "Persist on Error" (Durable Retries)

  Currently, your executeWithRetry does a time.Sleep in memory. If the process dies, the retry is lost.

  New Flow:
   1. Step A fails.
   2. Executor checks config. If Durability: Ephemeral, it sleeps in memory (current behavior).
   3. If Durability: Durable, the Executor stops execution.
   4. It enqueues a GraphExecutionArgs job into River with opts.ScheduledAt(time.Now().Add(backoff)).
   5. The in-memory Execute call returns a specific error: ErrJobOffloadedToQueue.

  Benefits:
   * You don't block a thread for 1 hour waiting for a retry.
   * If the server restarts, the retry still happens.

  Scenario B: Explicit Configuration (Async Steps)

  User configures a step to be a "boundary".

   1 g.Register(
   2     SendEmailStep,
   3     // Tell Docket this step should be run by a River worker, not inline
   4     docket.WithExecutionMode(docket.ModeAsync), 
   5 )

  Flow:
   1. Executor reaches SendEmailStep.
   2. It sees ModeAsync.
   3. It executes SendEmailStep.
   4. Crucial: To be safe, it should probably snapshot after the step completes, effectively "checkpointing" the progress to the database.

  Scenario C: Checkpointing

  Users might want to save progress after expensive computations.

   1 // Explicit checkpointing
   2 g.Register(HeavyComputationStep)
   3 g.Checkpoint() // Hypothetical API
   4 g.Register(NextStep)

  Flow:
   1. Executor finishes HeavyComputationStep.
   2. It hits the Checkpoint.
   3. It serializes the current cache (containing the heavy result).
   4. It enqueues a River Job to resume execution immediately.
   5. The current process exits.
   6. Another worker picks up the job, has the heavy result in cache, and starts NextStep.

  ---

  3. What we need to build

  To make this work, we need a "Durable Runtime" wrapper.

  1. Graph Registry
  Since we are serializing to JSON/DB, we can't pass func pointers. We need a way to look up the graph definition by name when the worker starts.

   1 // In your main.go / setup
   2 registry := docket.NewRegistry()
   3 registry.Register("MyDailyReport", MyDailyReportGraph)

  2. The River Worker
  We implement a River Worker that bridges the gap.

    1 type GraphWorker struct {
    2     Registry *docket.Registry
    3 }
    4 
    5 func (w *GraphWorker) Work(ctx context.Context, job *river.Job[GraphExecutionArgs]) error {
    6     // 1. Look up the graph definition
    7     graph := w.Registry.Get(job.Args.GraphName)
    8     
    9     // 2. Hydrate the cache from the job args (Proto Unmarshal)
   10     hydratedCache := DeserializeCache(job.Args.CacheSnapshot)
   11     
   12     // 3. Execute! 
   13     // We need to modify Execute to accept a "Pre-filled Cache"
   14     _, err := docket.Execute(ctx, graph, job.Args.ExecutionID, docket.WithHydratedCache(hydratedCache))
   15     
   16     // 4. Handle interruptions
   17     if errors.Is(err, docket.ErrJobOffloaded) {
   18         // The graph decided to sleep/retry/checkpoint and scheduled its own next job.
   19         // We just mark this current job as done.
   20         return nil
   21     }
   22     
   23     return err
   24 }

  Summary of Changes Required

   1. Serialization: Add helper functions to turn the executor.cache (map[reflect.Type]proto.Message) into map[string][]byte and back.
   2. Executor Update: Allow Execute to accept an initial, pre-populated cache (the snapshot).
   3. Return Values: Define a sentinel error ErrOffloaded so the caller knows "I didn't fail, but I moved to the database."
   4. River Module: Create a pkg/docket/durable package that contains the River Worker implementation and the Registry.

  This keeps the core pkg/docket pure (in-memory, standard library only), while pkg/docket/durable adds the Postgres/River capabilities for those who want
  them.

                 