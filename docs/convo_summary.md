## workflows vs jobs and commercial incentives

You're right. Let me think about this more clearly.

**The commercial pressure:**

To be a business, these tools have to offer something you'll pay for. That pushes them toward:
- Managed infrastructure (so you pay for hosting)
- Proprietary features (so you can't just self-host the open source version)
- Complexity that justifies the price (simple things don't command big contracts)
- Lock-in (so you keep paying)

The result is tools that do more than you need, in ways that create dependency.

**What each is actually selling:**

- **Temporal Cloud**: You can self-host, but scaling is hard enough that you'll eventually pay them
- **Inngest**: Creates queues, state, retries—but now your queues live in their cloud
- **Trigger.dev**: Dev/review/staging/prod environments—but useful lower environments with meaningful data are already hard without integrating someone else's cloud
- **Step Functions**: Pay per state transition—which incentivizes them to make everything a state transition
- **Restate**: Durable async/await—but if you don't need durability on every await, you're paying for nothing

**The core confusion:**

There's a conflation between two different things:
1. **Scheduled/delayed jobs**: "Do X in 30 minutes" or "Do X every day at 3am"
2. **Durable multi-step workflows**: "Do X, then wait for human approval, then do Y, resuming even if the server restarts"

Most of what people need is (1). They reach for workflow engines because they seem comprehensive, then pay the costs of (2) on everything.

If it's a timed job, use a job queue. If it's a durable workflow with long sleeps and complex coordination, maybe you need something heavier—but be honest about how often that actually comes up.

**What you actually want:**

- Job queue: Postgres-backed, transactional, queryable
- For the rare true workflow: build it explicitly with a state machine and a jobs table, or accept the tradeoffs of a workflow engine for just that use case

The mistake is adopting a workflow engine as general infrastructure when 95% of your work is jobs.

This description is ok, but misses the type resolution/creation piece. like, the goal of an RPC is to do side effects and 
create/return types. and your application is a bunch of rpcs

## The cost focused model

Cheap step that fails? Rerun it. Cost: milliseconds.
Expensive step you already completed? Read from cache. Cost: one database read.
Side effect you can't repeat? Persist it. Cost: one database write when it succeeds.


## a short pitch

Let me understand this better.

**Dependency resolution:**

So Protograph isn't just "run A, then B, then C." It's "B needs the output of A, C needs outputs of A and B, D needs only C." The framework resolves what needs to run based on what each step needs.

If you're retrying and C is already cached, and D only needs C, then A and B don't run at all. You don't replay the whole chain—you just fetch what's needed for the step you're on.

This is different from Temporal's replay model where you re-walk the whole history. It's lazy evaluation of a dependency graph.

**Guaranteed wiring:**

The protobufs aren't just for serialization—they're a contract. Step B declares it needs `ChargeResult`. Step A declares it produces `ChargeResult`. The framework ensures the wiring is correct at definition time, not at runtime.

You can't deploy a workflow where step B expects something step A doesn't provide. The types enforce it.

**Testability:**

Each step is a function with typed inputs and outputs. Test it in isolation: "given this `ChargeResult`, does `ReserveInventory` do the right thing?" No workflow harness, no replay infrastructure, no test server. Just unit tests.

The workflow itself is also testable: "given these cached values for steps A and B, does the workflow produce the right final result?" Mock the steps, verify the composition.

**Safe multi-team contribution:**

This is the platform story we said failed with Temporal. Why does it work here?

- Types enforce the contracts. A team can't break another team's step by changing output shapes—the compiler catches it.
- No shared runtime state. Each workflow execution is independent, using the shared database but not sharing in-memory state that could corrupt.
- No cluster to destabilize. A bad workflow from one team might fail repeatedly, but it's just a job in River. It doesn't cascade into other teams' work.
- Onboarding is small. "Here's how you define a step. It takes typed input, produces typed output. Here's how you mark something as cached. That's it."

**What else:**

- **Incremental computation.** If the inputs to step A haven't changed since last run, and A is cached, skip it. Like a build system for workflows.
- **Transparency.** The dependency graph is explicit and inspectable. You can visualize what depends on what, see where a workflow is stuck, understand the shape of the process.
- **Composability.** A workflow can be a step in another workflow. The dependency resolution works recursively.
- **Partial reruns.** "Rerun from step C" is trivial—fetch cached values for A and B, run C and everything downstream. No need to replay history.
- **Cheap by default.** Nothing is cached unless you say so. The default is recompute, which is usually the right default.

**The contrast with Temporal:**

| | Temporal | Protograph |
|---|---|---|
| Execution model | Replay history | Resolve dependencies, fetch or compute |
| Default | Persist everything | Persist nothing |
| Wiring | Runtime, string-based | Compile-time, typed |
| Testing | Needs replay harness | Unit tests |
| Multi-team | Risky, onboarding heavy | Safe, minimal onboarding |
| Retry | Replay from start | Fetch what's needed, continue |
| Partial rerun | Reset workflow to event ID | Just run from any step |

**The real pitch:**

Protograph isn't a workflow engine. It's a dependency resolver with optional caching, typed contracts, and lazy evaluation—running on your existing job queue and database.

It gives you the linear readability of workflow-as-code, but the execution model is "compute what you need, cache what you choose, let River handle retries." No replay, no determinism constraints, no operational burden.



## when you look at a competitor like temporal, what are you really missing

What you actually have:

A database that stores state
A job queue that handles retries, scheduling, concurrency
Services that do work
Types (protobufs, structs, whatever) that define your data

What you're missing:
A way to say: "This process has steps. These steps have dependencies. Some results are precious, some aren't. Figure out what needs to run."
That's it. That's the gap.
The paradigm:
It's not workflow orchestration. It's dependency resolution with selective caching.
Like a build system. Make doesn't recompile everything—it looks at what changed, what depends on what, and computes the minimum work. If foo.o is up to date and bar.c only needs foo.o, you don't recompile foo.c.
Protograph does this for multi-step processes. Define the steps, declare what each step needs, mark what's precious. When it runs—or reruns—it resolves the graph, fetches cached values where they exist, computes what's missing.
The pieces:

Step definitions. A step is a function with typed inputs and typed outputs. That's all.
Dependency declarations. Step B needs the output of step A. This is explicit in the type signature—B takes ChargeResult, A produces ChargeResult.
Selective persistence. Mark a step as cached when its result is precious (side effects, expensive computation). Unmarked steps recompute on retry.
A resolver. Given the target step and what's already cached, figure out the minimum work to get there.

That's the whole thing.
You don't need:

A replay system (just recompute or fetch)
A history service (state is in your database)
A workflow runtime (it's just functions)
Determinism constraints (steps can do whatever)
A versioning system (it's just code)
A new queue (River works fine)
A new database (Postgres works fine)

Why this is different:
Temporal models workflows as programs that replay. The runtime re-executes your code, skipping side effects by returning cached results. This requires determinism, history storage, versioning.
Protograph models workflows as dependency graphs with caching. The resolver figures out what to run. This requires only: typed steps, a cache, a resolver. Your existing infrastructure handles everything else.
The pitch, simplified:
You have a job queue. You have a database. You have typed data structures.
You're missing a dependency resolver that knows which steps need which inputs, and a cache that remembers precious results.
Add those two things. That's Protograph. Everything else you already have.

## short pitch

**What problem is it trying to solve?**

Multi-step processes accumulate state. Step B needs what step A produced. Step C needs what A and B produced. When something fails, you need to recover without losing work or repeating side effects.

That's it. That's the whole problem.

**Why is that the right problem?**

Because everything else is already solved.

Retries? Your job queue does that. Scheduling? Your job queue does that. Persistence? Your database does that. Types? Your language does that. Observability? Your existing tools do that.

What your job queue doesn't know is: "This job has internal steps. Some of those steps already succeeded. When you retry, don't redo everything—figure out what's still needed."

What your database doesn't know is: "These cached values are step outputs. This step depends on that step. Here's the graph."

What your types don't express is: "Step B requires `ChargeResult`. Step A produces `ChargeResult`. The wiring is correct by construction."

**The gap:**

Your infrastructure handles jobs. It doesn't understand that jobs have structure—steps, dependencies, intermediate results.

Protograph adds that understanding. It's a dependency resolver with selective caching. Nothing more.

**Why not Temporal?**

Temporal solves this problem by rebuilding everything: its own persistence, its own retry logic, its own scheduling, its own observability. Then it adds replay semantics that require deterministic code and careful versioning.

You're paying for an entire runtime to get dependency resolution.

**Why Protograph?**

You already have a runtime. You have Postgres. You have River. You have protobufs.

You're missing one thing: something that understands steps depend on other steps, and some results are precious.

Add that. Keep everything else.

## things you don't need creating other things you don't need

Let me trace the dependencies.

**Start with replay.**

Temporal replays your workflow code to reconstruct state. Every time a worker picks up a workflow, it re-executes from the beginning, using cached results to skip already-completed steps.

Replay requires:
- **Determinism.** If the code takes a different branch on replay, state reconstruction fails. No random numbers, no reading the clock, no non-deterministic iteration order.
- **History storage.** Every step's inputs and outputs must be stored so replay can return them. This is the opaque blob problem.
- **Versioning.** If code changes between executions, replay might take a different path. Now you need patching, version markers, compatibility checking.
- **A runtime.** Something has to execute the replay, manage the history, orchestrate the workers.

**Why does Temporal replay?**

Because it models workflows as programs that suspend and resume. Your code looks like a normal function with awaits—the framework makes it durable by recording every step and replaying to restore state.

The programming model is nice. The implementation requirements cascade.

**The cascade:**

```
"Workflows are programs that suspend and resume"
    → Replay to reconstruct state
        → Deterministic code required
        → Full history storage required
            → Opaque blobs (no schema)
            → Scaling problems
            → Custom database
        → Versioning system required
            → Patching complexity
            → Replay testing in CI
        → Custom runtime required
            → History service
            → Matching service
            → Worker polling model
            → Operational burden
```

**What if you don't replay?**

If you don't replay, you don't need determinism. Functions can do whatever—call the clock, use random numbers, iterate in any order.

If you don't replay, you don't need full history. You only store what you explicitly mark as precious.

If you don't replay, you don't need versioning. It's just code. Deploy it. Old jobs in the queue have old inputs; they'll run against new code. If that's a problem, it's the same problem you have with any job queue—and you already know how to handle it.

If you don't replay, you don't need the runtime. Your job queue retries jobs. Your database stores cached values. Your code is just functions.

**The artifact chain:**

Every "feature" Temporal has exists to support replay:

| You need this... | ...because of this |
|---|---|
| Deterministic code | Replay |
| History service | Replay |
| Opaque blob storage | Storing arbitrary step outputs for replay |
| Custom database | Scaling the history service |
| Versioning system | Replay against changed code |
| Replay testing | Catching non-determinism before prod |
| Worker polling model | The runtime that executes replay |

Remove replay, and the whole edifice collapses. You don't need any of it.

**What Protograph does instead:**

Don't replay. Just run the function again.

When you run again, the dependency resolver checks: what does this step need? Is it cached? Fetch it. Not cached? Compute it. Compute it recursively until you have everything.

No replay. No determinism requirement. No history service. No versioning system. No custom runtime.

Just: dependency resolution and selective caching. On top of your existing job queue and database.

**The reframe:**

Temporal's complexity isn't incidental. It's the logical consequence of choosing replay as the execution model. Every painful thing about Temporal—determinism, versioning, opaque storage, operational burden—traces back to that choice.

Protograph makes a different choice. Don't replay. Resolve dependencies, cache selectively, recompute the rest. The complexity disappears because the root cause is gone.


## considering other data types outside of proto discussion

What protobufs give you:

Schema for storage - cached values have a known structure, so your database can store them efficiently and you can query fields
Versioning story - schema changes are explicit, you can reason about compatibility
Cross-language - if you ever want workers in different languages
Compile-time type checking - the generated code catches wiring errors at build time
Efficient serialization - smaller payloads, faster encoding

I could... make protobufs the recommended path but not the only path. Have a protograph.Serializer interface:
gotype Serializer interface {
Marshal(v any) ([]byte, error)
Unmarshal(data []byte, v any) error
Schema(v any) (string, error) // for queryable storage, optional
}
Ship protograph/proto for the protobuf implementation. Ship protograph/json for people who want simplicity and accept the tradeoffs. Document the difference honestly.
The pitch can then say "we recommend protobufs for production because schema matters" without demanding it for getting started.


The type-based inference is the whole point. "Your compiler already checks that the types line up" - that's the selling point over YAML-configured workflow engines. Adding string-based wiring throws that away.


## attempts at design principles

**Docket Design Principles**

1. **Fill the gap, don't replace the stack.** You have a database, a job queue, monitoring, deployments. Docket adds step dependencies and selective caching. That's it.

2. **No parallel universe.** One versioning system (git). One storage system (your database). One queue (your queue). No shadow infrastructure with its own rules.

3. **Types are the contract.** Step dependencies come from function signatures. The compiler verifies wiring. No YAML, no DSL, no runtime discovery.

4. **CacheResult what's precious, recompute what's cheap.** Side effects and expensive calls get cached. Validation and reads just run again. The default is recompute.

5. **No replay, no determinism constraints.** Your code is just code. Read the clock, generate random numbers, iterate however you want.

6. **Schema over blobs.** Cached results have structure. Your database knows what it's storing. You can query it.

7. **Exit anywhere.** Request any type from the graph. Docket computes exactly what's needed to produce it, nothing more.

8. **Library, not platform.** No cluster. No service. No per-execution pricing. Import it, use it, deploy your app.

9. **Boring is good.** SQL to see what's stuck. Your existing monitoring for metrics. Standard deployments for new code. No new operational concepts.

10. **Build what you need.** Docket doesn't do fan-out, compensation, or long waits automatically. If you need them, you build them. You probably don't need them.
11. 
**1. Fill the gap, don't replace the stack.**

*The need underneath: Respect for accumulated investment.*

Teams have spent years building operational knowledge. They know why their Postgres is configured that way. They know which alerts are noise and which mean something. They've earned that knowledge through incidents. A tool that asks them to start over—even if it's "better"—disrespects that investment. Docket should feel like a natural extension of what's already working, not a referendum on past choices.

**2. No parallel universe.**

*The need underneath: Cognitive continuity.*

Every parallel system is a context switch. "How does versioning work here?" is a different question for git, for database migrations, and for Temporal's patching system. Each one has different failure modes, different mental models, different tribal knowledge. Humans have limited capacity for parallel mental models. Docket should use the concepts you already have, so the knowledge you've built transfers directly.

**3. Types are the contract.**

*The need underneath: Early, local feedback.*

The worst time to discover a wiring mistake is in production. The second worst time is in CI. The best time is in your editor, as you type. Types make errors visible immediately, locally, without running anything. You don't have to hold the whole system in your head—the compiler holds it for you. Docket should fail fast and fail obviously, before anything runs.

**4. CacheResult what's precious, recompute what's cheap.**

*The need underneath: Explicit tradeoffs.*

Most caching systems make the decision for you, or make you cache everything. But caching has costs—storage, invalidation complexity, stale data risk. You know which results are precious (charged a credit card) and which are cheap (validated an email format). Docket should let you express that knowledge directly, not guess at it. The decision about what to keep should be intentional, not implicit.

**5. No replay, no determinism constraints.**

*The need underneath: Code should be code.*

Determinism constraints are a tax on every line you write. "Can I call this function? Will it break replay?" That cognitive overhead accumulates. It discourages refactoring because you're not sure what's safe. It creates a two-tier system: normal code and workflow code. Docket should let you write normal code. The durability mechanism should be invisible in your business logic, not a constraint on it.

**6. Schema over blobs.**

*The need underneath: Debuggability when it matters most.*

Systems fail at the worst times, when you're tired, when the stakes are high. In those moments, you need to answer questions fast: What state is this order in? What did this step return? When did it run? Blobs make you deserialize, write scripts, squint at hex. Schemas let you query. Docket should make the 2am debugging session as short as possible.

**7. Exit anywhere.**

*The need underneath: Composable, reusable computation.*

A graph of steps is a reusable asset. The same preprocessing pipeline serves training, evaluation, and inference—just different exit points. If you can only run the whole graph, you duplicate work or build parallel systems. If you can exit anywhere, one graph serves many use cases. Docket should let you get value from subsets of your computation without restructuring everything.

**8. Library, not platform.**

*The need underneath: Proportional commitment.*

Platforms want all of you. They want to be your queue, your storage, your scheduler, your monitoring. The commitment is large and hard to reverse. A library asks for less: import it, use it where it helps, ignore it where it doesn't. The risk of trying is low. Docket should be easy to adopt incrementally and easy to abandon if it doesn't work out.

**9. Boring is good.**

*The need underneath: Transferable skills.*

Novel systems require novel expertise. If debugging requires learning a custom query language, or monitoring requires their proprietary UI, you're building skills that don't transfer. SQL transfers. Prometheus queries transfer. Git knowledge transfers. Docket should lean on tools people already know, so the skills they build remain valuable regardless of whether they keep using Docket.

**10. Build what you need.**

*The need underneath: Honest scoping.*

Frameworks that do everything encourage you to use everything. Features have gravity—once they exist, they attract usage, and usage creates dependency. But most workflows don't need sagas, don't need fan-out, don't need compensation. Providing them anyway creates surface area for bugs, learning curve, maintenance burden. Docket should do a small thing well and be honest that the rest is your job. The absence of a feature is also a decision.


The needs under those cluster into a few themes:

---

**Sovereignty**

You own your system. You've made decisions for reasons. You understand the tradeoffs you've made even if you can't always articulate them.

A tool that respects sovereignty asks: "What do you already have? How can I work with it?" A tool that doesn't says: "Forget what you know. Here's how we do things."

*The question for a new feature:* Does this require users to hand over control of something they currently own? If yes, don't build it—or make it optional and interoperable with what they already have.

---

**Legibility**

When something goes wrong, you need to see what happened. Not a stack trace from someone else's abstraction. Not a blob you can't inspect. The actual state, in terms you understand.

Legibility means using familiar tools (SQL, not a custom query language), familiar concepts (database rows, not "workflow history"), and familiar locations (your database, not their service).

*The question for a new feature:* Does this make the system harder to inspect with standard tools? If yes, reconsider. Every layer of indirection is a tax paid during incidents.

---

**Proportionality**

The commitment you make should match the value you get. If you're solving a small problem, you shouldn't have to adopt a worldview.

This is about risk management. Small bets are reversible. Big bets aren't. Libraries are small bets. Platforms are big bets. A tool that asks for proportional commitment lets you try it in one workflow, see if it helps, expand or abandon based on evidence.

*The question for a new feature:* Does this increase the minimum commitment required to use Docket? If yes, it should be a plugin or extension, not core.

---

**Continuity**

The things you already know should keep working. Your intuitions about databases should apply to Docket's storage. Your intuitions about functions should apply to steps. Your debugging habits should transfer.

Continuity reduces cognitive load. Every novel concept is a thing to learn, remember, and maintain mental models for. Reusing existing concepts—even if a custom approach might be "better"—respects the limits of human attention.

*The question for a new feature:* Does this introduce a concept that doesn't exist elsewhere in the user's stack? If yes, can you express it in terms of something they already know? If not, is the value high enough to justify the new concept?

---

**Honesty**

A tool should be clear about what it does and doesn't do. It should not imply capabilities it doesn't have. It should not expand to fill space just because competitors occupy that space.

Honesty means saying "Docket doesn't do compensation, here's how you'd build it" instead of shipping a half-baked compensation system. It means documentation that says "this is hard, you'll need to think about it" instead of pretending everything is easy.

*The question for a new feature:* Are we building this because users need it, or because it would make the feature list longer? Does this feature exist because the problem is real and common, or because other tools have it?

---

**A one-sentence test:**

*"Does this feature make Docket more useful while preserving the user's sovereignty, legibility, proportionality, continuity, and honesty?"*

Or more simply:

*"Does this help users solve their problem without asking them to become dependent on us?"*

If a feature passes, build it. If it fails, either don't build it, or build it as an optional extension that users consciously opt into.

The needs under those cluster into a few themes:

---

**Sovereignty**

You own your system. You've made decisions for reasons. You understand the tradeoffs you've made even if you can't always articulate them.

A tool that respects sovereignty asks: "What do you already have? How can I work with it?" A tool that doesn't says: "Forget what you know. Here's how we do things."

*The question for a new feature:* Does this require users to hand over control of something they currently own? If yes, don't build it—or make it optional and interoperable with what they already have.

---

**Legibility**

When something goes wrong, you need to see what happened. Not a stack trace from someone else's abstraction. Not a blob you can't inspect. The actual state, in terms you understand.

Legibility means using familiar tools (SQL, not a custom query language), familiar concepts (database rows, not "workflow history"), and familiar locations (your database, not their service).

*The question for a new feature:* Does this make the system harder to inspect with standard tools? If yes, reconsider. Every layer of indirection is a tax paid during incidents.

---

**Proportionality**

The commitment you make should match the value you get. If you're solving a small problem, you shouldn't have to adopt a worldview.

This is about risk management. Small bets are reversible. Big bets aren't. Libraries are small bets. Platforms are big bets. A tool that asks for proportional commitment lets you try it in one workflow, see if it helps, expand or abandon based on evidence.

*The question for a new feature:* Does this increase the minimum commitment required to use Docket? If yes, it should be a plugin or extension, not core.

---

**Continuity**

The things you already know should keep working. Your intuitions about databases should apply to Docket's storage. Your intuitions about functions should apply to steps. Your debugging habits should transfer.

Continuity reduces cognitive load. Every novel concept is a thing to learn, remember, and maintain mental models for. Reusing existing concepts—even if a custom approach might be "better"—respects the limits of human attention.

*The question for a new feature:* Does this introduce a concept that doesn't exist elsewhere in the user's stack? If yes, can you express it in terms of something they already know? If not, is the value high enough to justify the new concept?

---

**Honesty**

A tool should be clear about what it does and doesn't do. It should not imply capabilities it doesn't have. It should not expand to fill space just because competitors occupy that space.

Honesty means saying "Docket doesn't do compensation, here's how you'd build it" instead of shipping a half-baked compensation system. It means documentation that says "this is hard, you'll need to think about it" instead of pretending everything is easy.

*The question for a new feature:* Are we building this because users need it, or because it would make the feature list longer? Does this feature exist because the problem is real and common, or because other tools have it?

---

**A one-sentence test:**

*"Does this feature make Docket more useful while preserving the user's sovereignty, legibility, proportionality, continuity, and honesty?"*

Or more simply:

*"Does this help users solve their problem without asking them to become dependent on us?"*

If a feature passes, build it. If it fails, either don't build it, or build it as an optional extension that users consciously opt into.


**Impermanence**

Software is temporary. The tool you adopt today will be replaced, outgrown, or abandoned. Maybe in two years, maybe in ten. The codebase will be rewritten. The team will turn over. The requirements will shift until the original architecture no longer fits.

Tools that ignore this try to become permanent. They want to be the foundation, the thing everything else depends on. They make leaving expensive—through data formats, through concepts that don't exist elsewhere, through coupling that spreads.

Tools that respect impermanence make themselves easy to leave. Not because they want you to leave, but because knowing you *can* leave changes the relationship. You're not trapped. You're choosing to stay, today, because it's still working. That's a healthier relationship than lock-in.

---

This reframes some of the earlier themes:

**Sovereignty** isn't just about respecting what you have now. It's about preserving your ability to make different choices later. If Docket owns your workflow state in a proprietary format, you're not sovereign—you're tenant.

**Legibility** isn't just about debugging. It's about understanding the system well enough to migrate away from it. If the state is in your Postgres with a schema you understand, you can write a migration to whatever comes next. If it's in opaque blobs with meaning only Docket understands, you're stuck.

**Proportionality** is partly about impermanence. Small commitments are easier to unwind. A library you imported is easier to remove than a platform you deployed. The proportional tool respects that your future self might make different choices.

**Continuity** means the skills you build aren't Docket-specific. You're learning Postgres patterns, Go patterns, River patterns. If you leave Docket, that knowledge remains useful. You haven't spent years learning a dialect that only exists here.

---

*The questions for a new feature:*

- If a user wanted to stop using Docket tomorrow, would this feature make that harder?
- Does this feature store data in a way that's meaningful outside of Docket?
- Does this feature teach concepts that transfer, or concepts that only exist here?
- Are we building dependency, or are we building utility?

---

There's a business tension here. Lock-in is profitable. Switching costs protect revenue. A tool that's easy to leave is a tool that has to keep earning its place.

But that tension resolves if you believe the way to keep earning your place is to keep being useful—not to make leaving painful. Docket should be the tool you keep choosing, not the tool you can't escape.

---

Maybe the one-sentence test becomes:

*"Does this help users solve their problem today without making tomorrow's choices harder?"*

Or:

*"Would we be comfortable if users evaluated Docket every quarter and consciously decided to keep using it?"*

If the answer is yes, the feature respects impermanence. If the answer is "we'd rather they didn't think about it," that's a sign.



## the software keeper test

Ha, yes it is. "Would I fight to keep this tool if someone tried to take it away?" vs "Am I using this because switching would be painful?"

There's something honest about building software that has to survive the keeper test. It forces you to keep delivering value rather than accumulating leverage.

It also changes what you optimize for. Lock-in optimizes for making the exit expensive. The keeper test optimizes for making the stay valuable. Those lead to different features:

| Lock-in approach | Keeper test approach |
|------------------|---------------------|
| Proprietary data formats | Standard formats, your database |
| Concepts that only exist here | Concepts that transfer |
| Deep integration everywhere | Shallow integration, clear boundaries |
| Features you might need someday | Features that pay for themselves now |
| Documentation that teaches your world | Documentation that teaches the problem |
| Switching costs | Switching ease as a feature |

---

This might be worth stating directly as a principle:

**The Keeper Test**

Docket should survive a quarterly review where you ask: "Is this still earning its place?"

That means:
- The value should be obvious and ongoing, not front-loaded
- The cost of leaving should be low, so staying is a real choice
- The skills you build should transfer if you leave
- The data should be yours in a format you can use without us

We'd rather you stay because Docket is good than because leaving is hard.

---

This also gives you a nice answer when someone asks "why doesn't Docket do X?"

"Because X would make Docket harder to leave, and we want you to stay because we're useful, not because you're stuck."

That's a principled reason that's easy to explain and easy to evaluate new features against.

## Mean HN comments and the responses

> We evaluated something similar to this (homegrown, same basic idea) before going with Temporal. The thing that killed us: observability.
"Just use SQL" sounds great until you need to answer "why is this order stuck" and the answer involves five different steps across three retries with partial cached state. Temporal's UI shows you the event history. It shows you exactly what happened. Your approach gives you... tables you write queries against?
We built a UI. Then we built a query builder. Then we built visualization for the dependency graph. Then we'd accidentally rebuilt half of Temporal's UI, poorly.

> How do you handle optional dependencies? What if a step can run with or without some cached value?
How do you handle diamond dependencies? A depends on B and C, both of which depend on D. Do you run D once or twice? How do you express "B's D and C's D should be the same execution"?
The type-based dependency inference sounds clean until you hit any real graph complexity. At some point you need explicit declaration, and then you have the YAML you were trying to avoid.

> Also, "certified" model weights in Postgres? At what scale? My smallest models are 500MB. Are you serializing pytorch checkpoints as protobufs into a relational database?

docket solves this by enforcing a single function that can return a type, and only one type can be returned. 

> Docket should survive a quarterly review where you ask: "Is this still earning its place?"

This is a nice sentiment but it's also not how organizations work. The cost of evaluating whether to keep something often exceeds the cost of just keeping it. "Easy to leave" is a marketing claim that's almost never tested until you're in a crisis and then everything is hard.
I've seen plenty of "simple" libraries become load-bearing and impossible to remove because they touched every workflow in the system. Shallow integration doesn't help when the integration is wide.

> so its luigi

So crucially, at most/at least semantics need to be specified on the step, timeouts need to be clear, 
errors need a way to propagate all the way to workflow failures. There needs to be a cache interface. 
we can add retries because everybody needs them

They're solving different problems that look similar from a distance.

**Luigi** (Spotify, ~2012)

Luigi is for batch pipelines. "Every night, run this ETL: download logs, clean them, aggregate, write to warehouse." It's about task scheduling with dependencies—make sure task B runs after task A, don't re-run tasks whose output files already exist.

Luigi is for: "process yesterday's logs." Docket is for: "process this order, and if it fails halfway, pick up where we left off."


Flink is for: "process 100k events/second, windowed aggregations, joins across streams." Docket is for: "this order has five steps, step 3 failed, don't re-run steps 1 and 2."

---

**The confusion is understandable**

They all have:
- Tasks/steps/operators with dependencies
- Some notion of "don't redo completed work"
- Graphs of computation

But they're serving different shapes of work:

| Tool | Shape of work | Recovery model | Where it runs |
|------|---------------|----------------|---------------|
| Luigi | Batch pipelines on schedule | Output file exists? Skip. | Scheduler + workers |
| Flink | Continuous streams | Checkpoints, replay stream | Distributed cluster |
| Temporal | Long-running workflows | Replay from event history | Temporal cluster |
| Docket | Request-triggered multi-step jobs | Certified results in your DB | Your app, your job queue |



If you're doing batch ETL, use Luigi (or Airflow, or Dagster, or Prefect). If you're doing stream processing, use Flink. If you're processing individual multi-step requests and want to recover gracefully from failures, that's where Docket fits.



| Tool | What it's for | Where state lives | What you operate | Paid/proprietary features | Best when you... | Not ideal when you... |
|------|---------------|-------------------|------------------|--------------------------|------------------|----------------------|
| **Docket** | Request-triggered multi-step jobs with selective caching | Your Postgres, your schema | Nothing new—your existing app and job queue | None, library | Have Postgres and a job queue, want to add step coordination without new infrastructure | Want managed everything, need long-running sleeps, want fan-out primitives built-in |
| **Temporal** (self-hosted) | Long-running workflows with full replay | Cassandra/MySQL/Postgres (opaque blobs), their schema | Temporal cluster: history service, matching service, workers | None, but significant ops burden | Need human-in-the-loop waits, complex compensation, have platform team capacity | Don't have dedicated infra team, want queryable state, sensitive to latency |
| **Temporal Cloud** | Same as above, managed | Their cloud | Workers only (your code) | Per-action pricing, support tiers, SLAs | Want Temporal without operating Temporal, budget for managed service | Cost-sensitive at scale, data residency requirements |
| **Inngest** | Event-driven serverless workflows | Their cloud | Nothing—fully managed | Step runs, concurrency limits, log retention, priority queues | Want zero infrastructure, event-driven patterns, quick prototyping | Need data on-prem, want to avoid per-execution costs, need deep database integration |
| **Trigger.dev** | Background jobs with dev/staging/prod environments | Their cloud | Nothing—fully managed | Concurrent runs, team seats, run history retention | Want managed job infra with nice DX, TypeScript-native | Already have job queue you like, want state in your database |
| **Restate** | Durable async/await execution | Their service (self-host or cloud) | Restate server | Cloud hosting, support | Want durable execution with minimal code changes, async/await model | Don't want another service, need deep Postgres integration |
| **AWS Step Functions** | State machines for AWS services | AWS (DynamoDB-backed) | Nothing—AWS manages | Per-state-transition pricing, Express vs Standard pricing tiers | Already AWS-native, want visual state machine editor, integrating AWS services | Cost-sensitive with high transition volume, multi-cloud, want to avoid AWS lock-in |
| **Luigi** | Scheduled batch ETL pipelines | File system / S3 (output targets) | Scheduler, workers | None, open source | Running daily/hourly data pipelines, output is files, batch semantics | Processing individual requests, need sub-second latency, request-driven workloads |
| **Airflow** | Scheduled batch pipelines with complex dependencies | Postgres/MySQL (DAG state), your data stores | Airflow scheduler, webserver, workers | Managed offerings (Astronomer, MWAA) add support, SLAs, easier ops | Complex DAG scheduling, lots of integrations needed, have data platform team | Request-driven workflows, need low latency, simple dependencies |
| **Prefect** | Modern Airflow alternative, hybrid execution | Their cloud (orchestration), your infra (execution) | Prefect agent | Cloud tiers: automations, audit logs, SSO, collaboration | Want Airflow-like scheduling with better DX, hybrid model | Want fully self-hosted, no external orchestration dependency |
| **Dagster** | Data pipelines with software engineering practices | Your infra (self-host) or their cloud | Dagster instance, daemon | Dagster Cloud: managed infra, branching, insights | Data pipelines with strong typing, asset-oriented thinking, want good testing story | Not doing data pipelines, request-driven workloads |




The HN commenter asking "so it's Luigi?" is half-right
The dependency graph idea is similar. But Luigi assumes:

You're running pipelines, not processing individual requests
Your outputs are files on disk or S3
You have a scheduler orchestrating runs
Batch semantics—run the whole thing, or don't

Docket assumes:

You're processing individual workflows triggered by your application
Your outputs are rows in your database
Request semantics—one order, one user, one thing at a time

**What you're paying for (explicit and hidden):**

| Tool | Explicit cost | Hidden cost |
|------|---------------|-------------|
| Docket | None (library) | Your Postgres capacity, your job queue, building what's not included |
| Temporal self-hosted | None (open source) | Ops team time, Cassandra/DB costs, learning curve |
| Temporal Cloud | Per-action + retention | Vendor dependency, data in their cloud |
| Inngest | Per-step, concurrency tiers | Data in their cloud, ceiling on customization |
| Trigger.dev | Per-run, team seats | Data in their cloud, their environment model |
| Step Functions | Per-transition (adds up fast) | AWS lock-in, debugging in CloudWatch |
| Airflow | None (open source) or managed pricing | Ops burden, DAG complexity, scheduler as bottleneck |

 async/await with minimal code changes" → **Restate**
 

# Draft Blog Post announcing Docket

# Docket

People keep building workflow engines. There's Temporal, which came out of Uber's Cadence, which came out of Amazon's Simple Workflow Service, which came out of the observation that distributed systems are hard and computers crash sometimes. Then there's Inngest, Trigger.dev, Restate, Windmill—a whole ecosystem of companies solving the same problem with different packaging.

The pitch is always the same: write your business logic as a normal function, and we'll make it durable. If the server dies mid-execution, we'll pick up where you left off.

This is a real problem. If you're processing an order and the machine crashes after charging the credit card but before reserving inventory, you need to recover somehow. You can't just run it again—you'll double-charge the customer. You can't just skip it—they paid for nothing. You need to know what already happened and what still needs to happen.

So workflow engines solve this with replay. They record every step of your function's execution. When you recover, they re-run your function from the beginning, but instead of actually executing the steps, they return the recorded results. Your code thinks it's running normally; actually it's reconstructing state from a log.

This is clever. It's also a decision that echoes through everything else.

---

Replay has requirements. Your code has to be deterministic—if it takes a different path on replay, the recorded results don't match and everything breaks. So no random numbers in your workflow logic. No reading the current time. No iterating over hash maps in whatever order the runtime feels like. Your business logic is now constrained by the execution model.

And you need to store all those recorded results somewhere. Temporal stores them as opaque blobs, because it can't know ahead of time what types your code will use. Now your database is full of opaque blobs. You can't query them. You can't index them. Your database doesn't know what it's storing, so it can't help you store it efficiently.

And if your code changes—which it will, because you're a business and businesses change—you need a versioning system. The recorded history was made by the old code, but you're replaying with the new code. Temporal has a patching system for this: you mark your code with version flags, maintaining a parallel versioning system alongside git. You already know how to deploy code, run migrations, handle breaking changes. Now you have a second versioning system, specific to the workflow engine, with its own semantics and its own failure modes.

The workflow engine didn't just solve your problem—it gave you a new set of problems that happen to be the problems you need to have if you've bought into replay. It's a parallel universe. Their queues, their storage, their versioning, their execution model. You're not adding durability to your existing infrastructure; you're moving to a new country.

---

Here's what you already have:

**Queues.** RabbitMQ, SQS, Redis, Postgres-backed queues like River or Oban. Your queues work. They're monitored. Your team knows how to debug them.

**Retries with backoff.** Your job queue does this. Every job queue for the last fifteen years does this.

**Persistence.** You have a database. It has a schema. It knows what it's storing. It can index, compress, query.

**Observability.** You have Prometheus or Datadog or whatever. You have dashboards and alerts and runbooks.

**Versioning.** You have git. You have database migrations. You have deployment pipelines.

When you adopt a workflow platform, you don't get to trade in your existing infrastructure. You get to have two of everything—yours, which you understand and operate, and theirs, which you're now also learning.

---

So what's actually missing? What gap are the workflow engines filling?

Your infrastructure handles jobs. But it doesn't understand that some jobs have structure: steps with dependencies, where some results are precious and some are cheap to recompute.

A job queue sees a job as atomic. It runs or it doesn't. If it fails, you retry from the beginning. But your order processing isn't atomic—it's charge the card, then reserve inventory, then create the shipment. If it fails after charging the card, you don't want to retry from the beginning. You want to skip the charge (already done) and retry the inventory reservation.

That's the gap. Not queues, not retries, not persistence. Just: understanding step dependencies and knowing which results to keep.

---

Docket fills that gap.

You define steps as functions:

```go
func ChargeCard(ctx context.Context, req *ChargeRequest) (*ChargeResult, error) {
    // talk to Stripe
    return &ChargeResult{TransactionID: txn.ID, Amount: req.Amount}, nil
}

func ReserveInventory(ctx context.Context, req *ReserveRequest) (*Reservation, error) {
    // talk to your inventory service
    return &Reservation{Items: req.Items}, nil
}

func CreateShipment(ctx context.Context, charge *ChargeResult, inv *Reservation) (*Shipment, error) {
    // talk to your shipping provider
    return &Shipment{TrackingNumber: tracking}, nil
}
```

Notice that `CreateShipment` takes a `ChargeResult` and a `Reservation`. Those are its dependencies. Docket sees this in the type signature. You don't declare the graph in YAML or a DSL—it's implicit in the function signatures. Your compiler already checks that the types line up.

You register these steps and mark which ones are precious:

```go
d := docket.New(db)
d.Register(ChargeCard, docket.CacheResult())    // precious—don't double-charge
d.Register(ReserveInventory, docket.CacheResult()) // precious—don't double-reserve  
d.Register(CreateShipment)                   // not certified—cheap to retry
d.Register(ValidateOrder)                    // not certified—just validation
```

When you run:

```go
result, err := d.Run(ctx, orderID, CreateShipment, &ShipmentRequest{...})
```

Docket looks at what `CreateShipment` needs. It needs `ChargeResult` and `Reservation`. Are they certified (cached)? Fetch them. Not certified? Look at what *those* steps need, recursively. Build the minimum execution plan. Run it.

If the job fails after `ChargeCard` but before `CreateShipment`, and your job queue retries it, Docket fetches the certified `ChargeResult`, re-runs `ReserveInventory` if needed, and picks up where you left off. No replay. No determinism constraints. Just: what do we need, what do we have, what do we compute.

---

The key difference from replay-based systems: you're not recording a history and playing it back. You're caching specific results and recomputing everything else.

This means no determinism requirement. Your code can read the clock, generate random numbers, iterate in any order. It's just code.

It means no opaque blob storage. You define your types (Docket uses protobufs by default, but supports any serialization). Your cached results have a schema. Your database knows what it's storing.

It means no parallel versioning system. If your types change, you handle it the way you handle any schema change—with a migration. One versioning system, the one you already have.

And it means you stay in your existing infrastructure. Docket is a library. It stores cached results in your Postgres. It pairs with your existing job queue. There's no cluster to operate, no parallel universe to move to.

---

There's another property that falls out of this design: you can request any type from the graph and exit at that step.

```go
// Get me a ChargeResult for this order, computing whatever's needed
charge, err := d.Run(ctx, orderID, ChargeCard, &ChargeRequest{...})

// Different call: get me a Shipment, computing the full chain
shipment, err := d.Run(ctx, orderID, CreateShipment, &ShipmentRequest{...})

// Another call: just get the cached ChargeResult, no computation
charge, err := d.Fetch(ctx, orderID, ChargeResult{})
```

The graph is the same. You're just asking for different exit points.

This turns out to be useful for ML pipelines. A typical ML workflow has steps: load data, preprocess, extract features, train model, evaluate, deploy. Each step depends on previous steps. Some are expensive (training), some are cheap (evaluation).

```go
d.Register(LoadData)
d.Register(Preprocess, docket.CacheResult())
d.Register(ExtractFeatures, docket.CacheResult())  
d.Register(Train, docket.CacheResult())
d.Register(Evaluate)
d.Register(Deploy)
```

Now you can:

```go
// Full training run
model, err := d.Run(ctx, runID, Deploy, &DeployRequest{})

// Just get features for analysis—stops at ExtractFeatures
features, err := d.Run(ctx, runID, ExtractFeatures, &FeatureRequest{})

// Re-evaluate a trained model with new test data—fetches cached model, runs Evaluate
eval, err := d.Run(ctx, runID, Evaluate, &EvalRequest{TestData: newData})
```

Same graph, different exit points. The expensive steps (preprocessing, feature extraction, training) are certified, so they're computed once. The cheap steps (evaluation) recompute freely. You can re-run evaluation with different parameters without re-training.

For serving, you might only need inference:

```go
// Inference uses the trained model but doesn't need the training data
prediction, err := d.Run(ctx, runID, Predict, &PredictRequest{Input: input})
```

`Predict` depends on `Train` (needs the model weights). Docket fetches the certified model, runs inference. The data loading, preprocessing, feature extraction steps don't run—`Predict` doesn't depend on them, only on their output via the trained model.

---

Docket has a River plugin:

```go
import (
    "github.com/riverqueue/river"
    "github.com/docket/docket/riverdriver"
)

client, _ := river.NewClient(riverpgxv5.New(db), &river.Config{
    Queues: map[string]river.QueueConfig{
        "orders": {MaxWorkers: 100},
    },
})

d := docket.New(db, riverdriver.Plugin(client))
```

Now River handles scheduling, retries, concurrency, backpressure. Docket handles step dependencies, caching, resolution. Each does what it's good at.

```go
d.Enqueue(ctx, "orders", CreateShipment, &ShipmentRequest{OrderID: order.ID})
```

When River retries a failed job, Docket's resolver runs. Certified results are fetched. Uncertified steps are re-executed. The job picks up from wherever it needs to.

---

**What Docket doesn't do.**

Long waits. If you need to wait 30 days and then do something, that's a scheduled job—use River's scheduling. Complete your current process, schedule a new job for later. The state is in your database; the new job reads it.

Automatic fan-out/fan-in. If you need to process 100 items in parallel and aggregate, you enqueue 100 jobs, track completion, have the last one trigger aggregation. Docket doesn't manage that coordination for you.

Automatic compensation. If step 4 fails and you need to undo steps 1-3, you write that logic. You know your domain; you know what "undo" means.

A UI. Your database gives you a UI—it's called any SQL client, or Grafana, or Retool.

These are intentional limits. Each one is something workflow engines provide, and each one comes with complexity, opinions, and failure modes. Docket's bet is that you'd rather build the parts you actually need than pay for everything you mostly don't.

---

**What Docket does do.**

**Dependency resolution.** Steps declare what they need through their type signatures. Docket builds the graph and figures out what to run.

**Selective caching.** You mark what's precious with `CacheResult()`. Everything else recomputes.

**Flexible exit points.** Request any type from the graph. Docket computes exactly what's needed to produce it.

**Type-safe contracts.** Your compiler verifies step wiring. Schema changes are explicit migrations.

**Queryable state.** Everything is in your database, with a schema. SQL works.

**Your infrastructure.** Postgres for storage. River for queuing. Your monitoring for monitoring. Docket is a library, not a platform.

---

The workflow vendors built replay, then built everything replay requires: determinism checking, blob storage, history services, versioning systems. It's a coherent solution to the problem they defined.

Docket defines the problem differently: you have steps with dependencies, and some results need semantics or caching. That's a caching and resolution problem, not a replay problem. Solve it with caching and resolution.

Your database already stores state. Your source control handles versions. Your compiler already checks types. Add the part that understands step dependencies. Keep everything else.

