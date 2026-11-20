// Package sdk provides the MOI product-level Go SDK.
//
// Product SDK clients use the MOI product endpoint and product RBAC layer, not
// moi-core/catalog. Start user-facing tasks from the AI Decision Ladder, the shortest
// supported path from a user request to an SDK entry point, then use the User Story Matrix and AI Task Selection Guide
// in../docs/product-sdk-capabilities.md, use the AI Intent Disambiguation Matrix for
// overlapping user wording before choosing an entry point, use the User Context Entry Matrix
// to choose the SDK starting object from already available context, use User Story Ergonomic Usage Tiers
// to start with the minimal path before adding advanced control, use User Story Operation Intent Coverage
// to verify lifecycle action coverage, check User Story Capability Availability Coverage before choosing
// method names against the current endpoint/backend/features, use User Story Abstraction Fitness to keep abstractions
// user-shaped instead of route-group translations, use Cross-Domain User Journeys for outcome-level flows,
// check Cross-Domain Journey Example Anchors before copying README calls for multi-area outcomes,
// follow the User Story Flow Recipes for the ordered start point, next calls,
// produced handles/results, and source rules, check User Story State Handoff Coverage
// before passing values between SDK calls,
// check Cross-Domain Journey Risk Coverage
// before composing retries, cancellations, reruns, permission-sensitive operations,
// or external dependency calls, check User Story Discovery and Selection Coverage
// before constructing handles from ids,
// check User Story Selection Disambiguation Coverage before selecting one resource from list, search, name, or paginated results,
// check User Story Ordering and Recency Coverage before honoring latest, recent, oldest, first, top, sorted, priority, or time-window wording,
// check the User Story Outcome Contracts for task completion,
// check User Story Interaction Mode Coverage before polling, streaming, paginating,
// batching, deleting, cancelling, forcing, or asking for approval,
// check User Story Execution Budget Coverage before polling, streaming, paginating, retrying, or waiting on long-running work,
// check User Story Pagination and Partial Result Coverage before treating first pages, previews, streams, batches, or capped reads as complete,
// check User Story Error Recovery Coverage before retrying, prompting, rerunning, or adding fallback behavior,
// check User Story Permission and Scope Coverage before permission-sensitive calls or workspace/resource scope choices,
// check User Story Ownership and Collaboration Coverage before modifying, exposing, deleting, sharing, or continuing work on shared workspace resources,
// check User Story Input Construction Coverage before constructing specs, options, enum values, secrets, or payloads,
// check User Story Value Format and Units Coverage before serializing identifiers, timestamps, durations, byte sizes, limits, enum values, URLs, SQL, cron, or payload fields,
// check User Story Default and Omission Coverage before relying on omitted fields, empty values, zero values, or backend defaults,
// check User Story Completion Verification Coverage before treating writes, async starts, deletes, cancels, or error-only calls as complete,
// check User Story Result Accessor Coverage before reading returned values,
// check User Story Output Consumption Coverage before displaying or reusing returned values,
// check User Story Sensitive Data Handling Coverage before logging, persisting, showing examples, or passing SDK inputs/outputs into AI context,
// check User Story Automation Safety Coverage before repeating or automating SDK calls,
// check User Story Concurrent Submission Coverage before parallel calls, UI double-clicks, background polls, async retries, or multiple AI workers act on one task,
// check User Story Impact Assessment Coverage before asking for confirmation or executing high-impact SDK operations,
// check User Story Confirmation and Preview Coverage before destructive, cost-incurring, external, data-moving, permission-changing, or irreversible SDK operations,
// check User Story Abort and Cleanup Coverage before abandoning, canceling, compensating, or cleaning up partial SDK work,
// check User Story Progress Visibility Coverage before waiting on, streaming, paginating, batching, deleting, canceling, or otherwise monitoring SDK work,
// check User Story Diagnostic Handoff Coverage before handing failed or stuck tasks to a user, support engineer, log search, or later AI step,
// check User Story Freshness and Conflict Coverage before acting on previously read state,
// check the User Story Stability Evidence for facade tests, stable proof, and failure boundary,
// check User Story Representative Method Coverage and User Story Example Scenario Coverage before adapting examples,
// bootstrap with New and EnsureWorkspace,
// then use WorkspaceHandle descendants such as CatalogHandle, WorkflowHandle,
// ExploreSessionHandle, ConnectorHandle, ModelProviderHandle, ComputeResourceHandle,
// DataSharePublishHandle, and SQLWorkbookHandle.
//
// Route-aligned services remain available for explicit route-level control, but
// promoted product tasks should prefer the handle/spec facade. Existing ids
// needed by handles must come from typed SDK create/list/get results, UI-selected
// resources, or explicit user input; check the Handle ID Source Matrix before
// writing constructor calls, and do not invent ids or infer them from ProductData.
// Handle struct constructors and bind-only handle factory methods only bind
// already sourced ids; they are not discovery, create, or validation entry
// points and should not appear as copy anchors in generated examples.
// Example id variables and non-id inputs must also come from typed SDK results,
// UI state, secret storage, or explicit user input; check the Example ID Variable
// Source Matrix and Example Input Variable Source Matrix before filling snippet
// variables.
//
// Exported *Service types are route-aligned escape hatches for explicit
// route-level control. They are not user-story starting points; AI-written code
// should start from Client.EnsureWorkspace, WorkspaceHandle,
// and task handles/specs named by the User Story Matrix before using a service
// type directly.
//
// RouteMapping and RouteMappings expose route-alignment metadata for gates and
// diagnostics only. They are not user-story SDK entry points, AI task-selection
// sources, or example anchors.
//
// Exported result, envelope, stream, and protocol structs such as ProductData,
// StreamResponse, and *Envelope types are return values or typed request pieces,
// not SDK starting points or AI task-selection sources. AI-written code should
// choose Client or handle methods from the User Story Matrix, then consume
// these types only after a promoted method returns them.
//
// Option helpers such as WithAPIKey and WithTableDataPageSize only configure an
// already selected Client or handle method. They are not user-story SDK entry
// points, AI task-selection sources, or copy anchors by themselves.
//
// A2A method constants and low-level request structs such as A2AMethodSend,
// A2ARequest, and AgentA2ARequest are explicit protocol-control values. They are
// not user-story SDK entry points, AI task-selection sources, or copy anchors by
// themselves. Prefer AgentHandle.SendText, AgentHandle.StreamText,
// AgentHandle.GetTask, AgentHandle.CancelTask, AgentHandle.ResubscribeTask, and
// AgentHandle.SubmitInput for promoted agent workflows; use AgentHandle.A2A only
// when the caller intentionally needs route-level A2A control.
//
// Error, IsCode, and Err... sentinel values are diagnostic and explicit
// error-branch values. They are not user-story SDK entry points, AI
// task-selection sources, recovery-strategy sources, or input-construction
// sources. Choose the task from the User Story Matrix first, follow that task's
// User Story Error Recovery Coverage, then inspect errors only to report
// request_id/code/message or to execute a documented caller-owned branch.
//
// UploadFile... convenience methods are local file input helpers that read a local path only after the caller
// explicitly supplies that path. They are not AI file-discovery, local-filesystem
// probing, filename inference, or default upload entry points. Prefer UploadBytes...
// when the application already owns bytes; use UploadFile... only with a path
// sourced from UI file input, storage picker state, or explicit user input.
//
// StreamResponse values are live response bodies for explicit streaming,
// preview, or download calls. They are not task completion signals,
// AI task-selection sources, or copy anchors by themselves. Callers must consume
// the stream according to the selected task's Progress Visibility, Pagination
// and Partial Result, and Output Consumption coverage, then close the body.
package sdk
