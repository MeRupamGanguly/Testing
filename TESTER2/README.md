

Wave-Based Scheduling: Uses Cron expressions to trigger processing "waves" at specific intervals.

Worker Pool Pattern: Leverages Go routines and channels to process multiple tasks concurrently within a single wave.

Claim-Check Pattern: Uses PostgreSQL SKIP LOCKED to ensure horizontal scalability (multiple instances can run without processing the same task twice).

Exponential Backoff & Jitter: Automatically delays retries for failed tasks to prevent system congestion.

Dead Letter Queue (DLQ): Quarantines tasks that exceed max_attempts for manual inspection and "Resurrection."

Auditability: Every state change is tracked via updated_at and last_error fields.

The repository includes a ResurrectTask function. Use this to move a task from DEAD_LETTER back to PENDING after fixing a data or code bug.

Task Lifecycle
PENDING: Task is created and waiting for the next wave.

PROCESSING: A worker has claimed the task and is executing business logic.

COMPLETED: The task finished successfully (Terminal State).

FAILED: An error occurred. The task is scheduled for a retry based on backoff logic.

DEAD_LETTER: The task failed too many times and requires human intervention (Terminal State).





What I did yesterday:
"I implemented the Task Model and Repository Interfaces, along with the Repository functions required for backoff and retry logic. I also ensured that the system maintains a lock so that a single task cannot be picked up by multiple workers simultaneously."

What I am doing today:
"Next, I will implement the main Batch Worker Pool pattern and the Cron Scheduler. I will also be adding a Graceful Shutdown mechanism to ensure the service handles process exits safely."

What We Needed & Why
Task Model & Repository Interfaces: We needed these to define a strict contract for how data moves through the system, ensuring the engine remains decoupled from the database.

Backoff & Retry Logic: This is necessary to prevent the system from "spamming" a failing task, which saves CPU and prevents external services from being overwhelmed.

Multiple Worker Prevention (SKIP LOCKED): This was critical to ensure data integrity; without it, two workers might process the same payment or send the same email twice.

Worker Pool Pattern: We need this to limit the number of concurrent tasks, preventing the application from crashing due to memory exhaustion or too many open database connections.

Cron Scheduler: This provides the automated "heartbeat" needed to check the database for pending work at specific intervals.

Graceful Shutdown: This is required to make sure that if the server restarts, we finish processing the current tasks instead of cutting them off mid-execution, which would leave them stuck in an inconsistent state
