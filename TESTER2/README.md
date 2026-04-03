# Go Concurrent Batch Scheduler

A high-performance, fault-tolerant Go application engineered to handle batch processing using a localized worker pool pattern and a PostgreSQL relational database. This system aggressively minimizes deadlocks and optimizes concurrency using native database row locking.

## 📖 What This Package Does

This package is a continuous cron-driven batch processor. It queries a PostgreSQL table for "Pending" or "Failed" tasks, securely claims them, and offloads them dynamically to a fixed-size worker pool running in concurrent Go routines. 

Instead of overwhelming your server by spawning a goroutine for every single payload, it caps resource usage at a defined rate while maximizing the efficiency of your active database connections.

---

## 🚀 How to Run and Test This Package

Since this application is tailored to run in an isolated, daemonless environment on Fedora, we use **Podman** to orchestrate both the Go execution and the database server safely.

### Automatic Test Suite
We have constructed a full automated testing script (`scr.sh`) that spins up the whole stack, seeds fake tasks, and validates completion.

1. **Ensure the setup script is executable:**
   ```bash
   chmod +x scr.sh


🛠️ Features and Functions
Smart Worker Pool: Restricts task processing to an explicitly defined boundary of workers to protect your system from exhausting memory or exhausting active DB connections.

Database Driven Concurrency Management: Uses the advanced Postgres operation FOR UPDATE SKIP LOCKED. This guarantees multiple separate instances of this app can run side-by-side against the same DB table without processing duplicate tasks or causing table deadlocks.

Cron Scheduled Triggers: Utilizes the lightweight robfig/cron/v3 library to effortlessly maintain rigid execution boundaries without external OS timers.

Exponential Random Backoff: Failed tasks are intelligently calculated and pushed back in time with a safe randomized margin so API limits aren't crushed during simultaneous retries.

Manual Dead Letter Resurrection: Tasks exceeding max attempts naturally fall into a DEAD_LETTER safety state. The application exposes a programmatic pathway to resurrect them back to active PENDING states after system administrators analyze the failure payloads.

📦 Dependencies Required
This package targets modern Linux ecosystems like Fedora and utilizes minimal external modules to maximize static binary compilation security.

System OS Dependencies
Podman (Daemonless containerization engine natively shipped with Fedora)

Bash (To execute automation shells)

Go Dependencies
Go 1.26 or higher (Required as specified by the strict toolchain bounds in the go.mod directive)

github.com/lib/pq (Pure Go Postgres driver)

github.com/robfig/cron/v3 (Thread-safe execution scheduler)

Database Dependency
PostgreSQL 12+ (Or a Docker/Podman library running postgres:latest with the pgcrypto extension activated for safe UUID generation)



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
