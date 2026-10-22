# Explicit execution bindings

An operator can bind a selected service to instructions, run hooks and a source
authority accepted by a compatible core. This is optional; unbound schedules keep
their existing execution and reusable-session fingerprint. The upstream core
without `access_context` support cannot execute a bound schedule.

```toml
[execution.bindings.report-delivery]
profile = "reports"
template = "native"
instructions_file = "/etc/space/report.md"
authority_namespace = "scheduler:space"
before_run = "prepare-report"
after_run = "deliver-report"
hook_services = ["report-delivery-secret"]

[execution.bindings.report-delivery.owners."alice@example.test"]
scope_context = { agent_id = "reporter", actor_user_id = 42, reader_user_ids = [42, 43] }
```

The key `report-delivery` must be a selectable core service. Selecting it is an
execution capability. Exactly one binding can match a schedule. Its stored
profile and template must match, and its normalized owner must have an explicit
operator-owned mapping. Missing or ambiguous mappings stop preparation before
calling core. Never infer an actor, readers or data permissions from an email,
prompt, display name or originating messenger.

Bearer keys still grant full Space API access. Do not distribute an unrestricted
Space key to an untrusted agent to enforce these rules. Use a scoped gateway or
another trusted source to restrict schedule owners, service capabilities and
destinations. A configured owner mapping does not verify who created a schedule.

Each occurrence freezes a new access context in its persisted request: the
occurrence UUID is the invocation ID, the schedule UUID is the subject ID, and
the configured namespace identifies the source. The configured scope is opaque
to Space. The context observation time describes this source's configured
audience; it is not an observation of external messenger membership. A delivery
integration must validate its actual destination independently.

Instructions and hooks enter the session configuration. `hook_services` enter
the request's run-only `services`, not sandbox services, so their credentials
must remain unavailable to the model. This requires a compatible core with
hook-only run services. The hooks receive `ORPHEUS_SPACE_CONTEXT` containing
schedule ID, occurrence ID, owner and scheduled UTC time. They also receive the
normal core run/session IDs. Hook services are resolved afresh for each run.

The reusable-session fingerprint includes the configured scope, namespace,
owner, hook services and effective instructions/hooks. Changing any of these
starts a fresh session. A new occurrence ID and observation time do not change
the fingerprint. Normal snapshot persistence, idempotency and unknown-outcome
reconciliation remain in effect; retries never manufacture new source authority.
