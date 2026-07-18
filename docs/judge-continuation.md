# Judge Session Continuation

When an Obey-managed agent runs a Fest approval judge, Fest resumes the exact
originating agent session after the judge reaches a terminal verdict. The
resumed turn carries the result, concise feedback, and the next valid Fest
command, so the agent can continue the loop without a human relaying the
outcome.

This is the Fest producer side. The Obey daemon side (delivery, the
`obey session notify` command, and env propagation) is documented in the Obey
repository under `docs/judge-continuation-notifications.md`.

## How It Works

1. When Obey starts a managed agent session, it exports `OBEY_SESSION_ID` and
   `OBEY_CAMPAIGN_ID` into the agent's environment.
2. Fest reads those values once, at judge launch, and pins them into the
   detached judge payload. The detached judge uses the pinned values and never
   re-reads the environment, so a changed child environment cannot redirect a
   continuation.
3. After the judge records a terminal verdict, Fest renders a continuation
   message and submits it to the originating session with `obey session notify`
   (JSON over stdin). Feedback is never interpolated into shell syntax.

Delivery is idempotent: the delivery id is derived from the judge run id, so a
retried or replayed terminal event resolves to a single agent turn.

## The Continuation Message

The message ends with the valid next command for the verdict:

- **Approved.** Confirms the step and points at `fest next`.
- **Rejected.** Includes concise feedback and ends with `fest workflow judge`.
- **Failed.** Includes concise detail and ends with `fest workflow judge`.

The next command is advisory text derived from the actual workflow state. It is
never a command Fest runs on the agent's behalf, and a notification never grants
approval authority.

## Continuity Prerequisite

Automatic continuation requires a kept or reusable Obey session. A session that
was intentionally stopped is not a valid target in the current version, and Fest
falls back to the manual next command.

## When Continuation Does Not Apply

Continuation is strictly additive. It is skipped, with existing behavior
preserved, when:

- `OBEY_SESSION_ID` (or `OBEY_CAMPAIGN_ID`) is absent. Standalone Fest and
  non-Obey agent workflows are unchanged and make no notification attempt.
- The `obey` binary is unavailable, or the daemon cannot accept the request. The
  judge result is still recorded and authoritative; Fest prints the manual
  command to continue.
- Delivery fails for any reason. A notification failure never becomes a judge
  failure and never changes workflow state.

## Troubleshooting

- **The agent was not resumed after a verdict.** Confirm the agent was launched
  by an Obey-managed session on a shell-capable provider, so `OBEY_SESSION_ID`
  was present at judge launch. Run the environment check inside the agent
  session: `echo $OBEY_SESSION_ID`.
- **Fest printed a manual command instead of resuming.** This is the fallback
  path. It means no session identity was captured, the `obey` binary was
  unavailable, or the target session was stopped. The judge result is still
  authoritative; run the printed command to continue.

## Release Compatibility

An updated Fest binary runs without an updated Obey daemon: when the notification
path is unavailable, Fest keeps its existing behavior and prints the manual next
command. Automatic continuation requires both sides to support the notification
schema, so upgrade the Obey daemon to enable delivery.
