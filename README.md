# Fees API with Encore and Temporal

This project implements a robust Fees API using the [Encore.dev](https://encore.dev/) framework for the backend and [Temporal.io](https://temporal.io/) for orchestrating long-running, reliable billing workflows.

It is designed to handle both usage-based and monthly billing cycles, allowing for the progressive accrual of fees (line items) and ensuring the durability and consistency of all financial operations.

## Project Overview

The core of the system is a long-running Temporal workflow, `BillLifecycleWorkflow`, that represents the lifecycle of a single bill. This workflow is responsible for:

- Tracking a defined billing period (e.g., one month).
- Accruing line item totals in its memory state via signals.
- Persisting each line item to a database for a historical record.
- Automatically closing the bill at the end of the period using a durable timer.
- Triggering a decoupled child workflow for post-processing tasks like generating invoices and sending emails.

## 1. Setup and Running

### Prerequisites

- **Go:** Version 1.18 or higher.
- **Docker:** Required by Encore to run the local Postgres database.
- **Temporalite:** A single-binary, zero-dependency distribution of Temporal for local development.

### Installation & Setup

1.  **Install Encore:**

    ```bash
    # macOS
    brew install encoredev/tap/encore

    # Linux
    curl -L https://encore.dev/install.sh | bash

    # Windows
    iwr https://encore.dev/install.ps1 | iex
    ```

2.  **Install and Run Temporalite:**
    Follow the official instructions at [temporal.io/temporalite](https://temporal.io/temporalite) to install it. Once installed, start the local Temporal service:

    ```bash
    ./temporalite start --namespace default
    ```

    Keep this terminal window open.

3.  **Run the Encore Application:**
    In a separate terminal, from the project's root directory, run:
    ```bash
    encore run
    ```
    Encore will start the service, connect to the local Postgres database (run via Docker), and connect to the local Temporalite instance. You will see a **Development Dashboard URL** in your terminal.

## 2. API Reference

You can interact with the API via `curl` or by using the **API Explorer** in the Encore Development Dashboard (usually at `http://localhost:9400/`).

### Create a New Bill

Starts a new usage-based billing cycle for a given `bill_id`.

**Endpoint:** `POST /bills`

**`curl` Example:**

```bash
curl -X POST http://localhost:4000/bills \
-H "Content-Type: application/json" \
-H "X-Idempotency-Key: $(uuidgen)" \
-d '{
  "bill_id": "project-xyz-usage",
  "billing_period_end": "'$(date -v+10M +%Y-%m-%dT%H:%M:%SZ)'"
}'
```

_(Note: The `date` command above is for macOS/BSD to get a timestamp 10 minutes from now. Adjust for your shell.)_

**How it Works:**

- A request to this endpoint triggers the `BillLifecycleWorkflow` with a `usage_based` policy type.
- The `X-Idempotency-Key` header is handled by Encore's middleware to prevent creating duplicate workflows from retried API calls.
- The `billing_period_end` tells the workflow when to automatically close itself.

### Add a Line Item

Adds a fee to an existing, open bill.

**Endpoint:** `POST /bills/{billID}/line-items`

**`curl` Example:**

```bash
curl -X POST http://localhost:4000/bills/project-xyz-usage/line-items \
-H "Content-Type: application/json" \
-H "X-Idempotency-Key: $(uuidgen)" \
-d '{
  "amount": 500,
  "currency": "USD",
  "description": "API Usage Charge"
}'
```

**How it Works:**

- This sends an `AddLineItem` signal to the running workflow.
- The workflow executes an activity that inserts the line item into the database. A unique `uid` is generated for this insertion to provide database-level idempotency.
- If the insertion is successful, the workflow updates its internal in-memory total for that currency.

### Manually Close a Bill

Explicitly closes a bill before its scheduled `billing_period_end`.

**Endpoint:** `POST /bills/{billID}/close`

**`curl` Example:**

```bash
curl -X POST http://localhost:4000/bills/project-xyz-usage/close
```

**How it Works:**

- Sends a `CloseBill` signal to the running workflow.
- The workflow stops its timer, finalizes the bill state by persisting the in-memory totals to the database, and triggers the post-processing child workflow.

### List Bills

Retrieves a paginated list of open or closed bills.

**Endpoint:** `GET /bills`

**`curl` Example (get open bills):**

```bash
curl "http://localhost:4000/bills?status=open&limit=5"
```

**How it Works:**

- This endpoint queries the database to get a list of bills.
- **Hybrid State Model:**
  - For **closed** bills, the total charges are read directly from the historical data in the database.
  - For **open** bills, it performs a **Temporal Query** against the live running workflow to fetch the real-time, up-to-the-second totals from its memory.

---

## 3. Architectural Decisions

This project includes several key design patterns that are critical for building robust, scalable financial systems.

### 1. Hybrid State Management

- **What:** The system uses a hybrid model for state. Live, in-flight totals are held in the workflow's memory for speed and consistency. Historical, finalized totals are persisted to the database for long-term storage and rich querying.
- **Why:** This provides the best of both worlds: the real-time accuracy of a Temporal Query for open bills, and the powerful query capabilities of SQL for historical reporting and listing.

### 2. Multi-Layer Idempotency

- **What:** Idempotency is handled at two layers. The Encore API gateway handles an `X-Idempotency-Key` for API-level retries. The `AddLineItem` activity also generates a unique ID (`uid`) for every database insertion, using `ON CONFLICT DO NOTHING` to prevent duplicates at the data layer.
- **Why:** This creates a robust defense against duplicate operations. API retries are stopped at the edge, and even if an internal error caused an activity to re-run, the database constraint would prevent a duplicate charge.

### 3. Using `BIGINT` for Currency

- **What:** All monetary values are stored as `BIGINT` in the database.
- **Why:** This is a best practice for handling money. It represents the smallest unit of a currency (e.g., cents for USD) as a whole number, completely avoiding floating-point precision errors which are a common source of bugs in financial calculations.

### 4. Decoupled Post-Processing with a Child Workflow

- **What:** After a bill is closed, the parent workflow starts a "fire-and-forget" child workflow to handle secondary tasks like PDF generation and emailing.
- **Why:** This decouples the critical financial transaction (closing the bill) from non-critical downstream operations. If the email service is down, it should not prevent the bill from being closed. The parent workflow completes quickly, and the child workflow can retry its tasks independently. This is achieved by setting `ParentClosePolicy: ABANDON`.

### 5. Temporal Timer for Bill Closure

- **What:** The workflow uses a durable `workflow.NewTimer` to automatically close the bill at its `billing_period_end`.
- **Why:** This is far more reliable than an external cron job. The timer is part of the workflow's persistent state and is guaranteed by Temporal to fire at the correct time, even if the application crashes and restarts. This makes the billing cycle self-contained and fault-tolerant.

---

## 4. Future Considerations

As a production-grade service, the following areas would be the next logical steps for improvement.

### Monitoring and Alerting

- **Problem:** If the `ClosedBillPostProcessWorkflow` fails after all retries, the system currently only logs an error.
- **Solution:** Integrate a proper monitoring solution. This could involve:
  - Emitting metrics to a system like Prometheus.
  - Forwarding structured logs to a service like Datadog or an ELK stack.
  - Setting up alerts (e.g., in PagerDuty or Slack) for critical failures, such as the inability to start a child workflow or a child workflow failing permanently.

### Enhanced Testing

- The project includes some unit tests, but a comprehensive test suite would include:
  - **Integration Tests:** Using the Temporal test framework (`TestWorkflowEnvironment`) to run the full workflow and activity lifecycle in a simulated environment, verifying interactions and final state.
  - **End-to-End Tests:** Scripts that call the live API endpoints and verify the behavior of the complete system.

### Configuration Management

- Currently, some values like timeouts and retry policies are hardcoded. These should be externalized into a configuration file or service so they can be tuned per environment (local, staging, production) without requiring a code change.

### Security

- The current API endpoints are public. In a real-world scenario, they would need to be protected. This would involve:
  - Implementing an `//encore:api auth` handler.
  - Defining authorization logic (e.g., only an authenticated service or user can add a line item to a bill).
