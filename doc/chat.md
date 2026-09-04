# Chat Architecture

## Purpose

Job match chat is a server-owned agent for one job application. It discusses a
job using the candidate's profile and application resume, and can create
validated application-resume revisions through a constrained tool.

The browser controls the agent, observes its persisted history, and can stop
the active turn. It does not own provider execution.

## Ownership And Lifetime

`jobs.service` owns a `JobMatchChatWorker`.

- One worker turn is active for a job at a time.
- Different jobs may run independently.
- A turn uses a context owned by the chat worker, not an HTTP request context.
- Closing a tab, changing panels, or losing an event subscription does not
  cancel a turn.
- Stop is an explicit command that cancels the active turn's context.
- Revert cancels affected active work, waits for it to stop, then removes the
  reverted conversation branch and its derived state.
- Service shutdown cancels active turns and waits for worker cleanup.

The active worker registry is process-local. It maps a job to its active
request ID, cancellation function, and completion signal. It is execution
control only, never a second copy of chat state.

No chat work is resumed automatically after a service restart. A restarted
service has no active workers. A user message without a completed reply remains
visible at the conversation tip. The user removes it with the existing U-turn
action before continuing the conversation; the UI does not provide a retry
control.

## Durable State

SQLite is the source of truth for all state that must survive a refresh,
navigation, or service restart.

### Conversation Items

`job_match_chat_items` is the one append-only, ordered record for a job chat.
Each item has a job ID, sequence number, type, JSON payload, request ID, and
timestamp. The sequence is unique within a job.

This is the only chat-specific durable table. It replaces separate chat-message,
agent-event, application-resume, and application-resume-revision tables. The
base resume, user profile, jobs, matches, and analyses remain separate
application data.

Items are the durable provider transcript and the agent trace. They include:

- immutable initial instructions and application context
- revision 0, the base application-resume snapshot
- user messages and assistant messages
- provider request and response metadata
- provider reasoning summaries and compatibility data
- assistant tool calls, tool results, and patch retry feedback
- successful tool results carrying application-resume revisions
- terminal turn outcomes and error records

The UI derives its conversation view by selecting user and assistant message
items. It derives revision history from accepted tool-result items carrying a
resume revision. It derives activity by selecting reasoning, tool, and
lifecycle items. All are views of the same ordered history, not independently
maintained sequences.

The item payload retains application-relevant provider artifacts without
forcing every event type into dedicated columns. It includes the effective
model, reasoning configuration, selected response, finish reason, usage,
provider metadata, refusal, tool arguments, tool outcomes, and supported
reasoning artifacts.

The item log never stores credentials, authorization headers, or unrestricted
HTTP debug data. Provider reasoning is model-specific: the UI presents a
provider-supplied summary or other safe explanation. Opaque or encrypted
reasoning details may be retained for provider compatibility, but are not
treated as normal assistant text.

An unanswered user-message item is valid state. It does not imply a background
worker is still running. A running worker is identified only by the active
worker registry in the current service process. After a restart, the existing
U-turn action removes an unanswered tip before the user continues.

### Immutable Context And Revisions

The first conversation items establish an immutable provider prefix: agent
instructions, job, match, profile, tool definitions, and revision 0 of the
application resume. These items are not rebuilt or changed on later turns.

A successful `revise_application_resume` tool call appends its raw assistant
tool-call item and an accepted tool-result item. That tool result carries the
full resulting resume snapshot, revision number, and summary, and is itself
the immutable revision record. Revision snapshots use stable nested resume IDs
so semantic changes can be computed from consecutive revisions. Structural
diffs are computed on demand and are not stored separately.

The latest revision is the only valid patch target, but every earlier revision
remains in the provider transcript. A new revision appends to the history; it
never replaces or hides an earlier revision.

A revision is created only after complete tool-call arguments have been
received and validated against the current revision. A partial or rejected
patch never mutates the application resume.

## Turn Execution

Starting a turn appends a user-message item, registers the active worker, and
returns without waiting for the provider.

The worker converts the immutable ordered item history into provider messages.
It does not rebuild or replace the initial system context. It appends every
provider-visible result in protocol order before making the next provider call.
It keeps in-flight continuation state in local memory:

- the assembled provider request
- provider responses
- tool calls and tool-result feedback
- retry-specific messages
- a proposed resume value before it is committed

This local state is not a cache or a competing state store. It exists only to
continue the current provider turn, including up to three resume-patch retry
attempts. If the worker stops or the service restarts, it is discarded. The UI
does not provide a retry action for an unanswered message.

The worker records relevant trace events as it progresses. It may expose live
transient output to connected clients. If partial assistant text is persisted,
it is clearly marked as incomplete or stopped and is not reused as a completed
assistant answer in later model context.

### Reasoning And Tool Retries

When a provider returns reasoning, the worker appends an
`assistant_reasoning` item immediately before the assistant message or tool
call it informed. The item retains a provider-supported summary and, when
needed for compatible replay, the original structured reasoning data. It is a
separate timeline item for inspection, although the provider serializer may
merge it into the associated assistant message when the provider protocol uses
reasoning as an assistant-message field.

For a resume patch attempt, the ordered item pattern is:

```text
assistant_reasoning
assistant_tool_call
tool_result
```

An accepted tool result includes the new full resume revision. A subsequent
provider completion may append reasoning and a user-facing assistant message.

A rejected tool result includes the validation error. The worker appends the
rejection in protocol order and makes a new provider call with the prior tool
call and tool result in context. It attempts at most three patches. After the
third rejection, it appends `retry_limit_reached` followed by `turn_halted`.
It does not append a synthetic assistant apology or request another provider
completion. The user can send a later message to start a new turn.

## Serialization And Consistency

All state-changing chat operations are serialized per job:

- A second send cannot run concurrently with an active turn for the same job.
- Stop can cancel a turn immediately without waiting for its execution lock.
- Revert cancels first, then waits for the worker before changing the tip.
- The base resume is snapshotted in the initial item prefix; later edits to the
  base resume do not change the application resume for that job.

The worker can read the durable item history once because no other operation
may change that job's chat branch while it is running. Its local retry
conversation is therefore safe for the lifetime of the turn.

Reverting to a user message deletes that item and every later item in the same
job sequence. This deliberately removes the derived assistant messages, tool
activity, and application-resume revisions in that branch. No orphaned
chat-derived state remains.

## Client Behavior

The client sends commands and reads state. It does not hold provider work open
through one long HTTP request.

- Send submits a user message and receives an accepted turn promptly.
- Stop calls the explicit server cancellation command for that request.
- Leaving chat stops polling or event subscription only.
- The client reloads conversation items from SQLite after refresh or
  navigation, then derives messages, revision history, and activity views.
- While a local worker is active, the client may poll its current activity or
  subscribe to live updates.
- When no worker is active after a restart, an unanswered tip remains visible
  until the user removes it with U-turn. It is never silently resumed.

The UI presents a focused subset of the trace: normal messages, agent status,
reasoning summaries, tool activity, validation feedback, and revision changes.
Detailed provider data remains available for inspection without cluttering the
conversation.

## Live Updates

The server delivers live chat updates with Server-Sent Events (SSE). The stream
delivers committed conversation items in order while normal HTTP commands send
messages, stop active work, and perform U-turn.

SQLite remains the source of truth. Every SSE item carries its durable sequence
position. On connection, reconnection, refresh, or a missed live notification,
the client loads every item after its last known position from SQLite before
continuing the stream.

The in-memory server notification only wakes live subscribers after an item has
been committed. It holds no chat history and may be missed safely because the
client always repairs gaps from SQLite. Closing an SSE connection stops live
delivery to that client only; it never stops the server-owned worker.

Provider token streaming may later add transient SSE deltas for responsive text
display. Durable conversation items remain the replayable history.
