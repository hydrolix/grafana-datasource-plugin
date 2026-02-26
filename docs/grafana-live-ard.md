# Architecture Decision Record: Error Surfacing via Grafana Live

## Status

**Proposed** February 2026

## Context

We need a mechanism to surface errors originating from Grafana dashboards and plugins back to users in near real-time.

One option under consideration is using **Grafana Live**, which provides WebSocket-based real-time messaging channels. While Grafana Live is designed for streaming data and live updates, its suitability for our specific use case—error surfacing tied to individual dashboard interactions—is questionable.

Our current approach relies on **Grafana dashboard variables combined with the Business Text plugin** to display error information. This solution is acknowledged to be somewhat hacky, but it has well-understood behavior and fits our needs reasonably well today.

## Decision Drivers

- **Operational simplicity**: Avoid complex proxy and networking requirements
- **Correctness of error attribution**: Errors must be scoped to the correct dashboard instance
- **Multi-user safety**: Avoid cross-talk between users or browser tabs
- **Architectural fit**: Prefer solutions aligned with intended feature use cases
- **Cost vs benefit**: Engineering effort must be justified by clear gains

## Considered Options

### Option 1: Use Grafana Live (WebSocket-based error delivery)

Leverage Grafana Live channels to publish errors from the backend plugin and surface them in dashboards via WebSocket connections.

#### Configuration Concerns

- Grafana deployed behind a proxy requires explicit WebSocket forwarding
- Risk of ephemeral port exhaustion under load (as documented by Grafana)
- Additional operational complexity compared to variable-based approaches

#### Architectural Concerns

- Only the **backend** part of a plugin can publish to Live channels
- Errors from different dashboard instances (browser tabs) are published to the same topic
- If multiple users open the same dashboard multiple times, all errors are mixed in a shared channel
- Routing errors to a specific dashboard instance might be possible but does not appear trivial
- Grafana Live lacks native per-dashboard-instance or per-tab isolation

#### Conceptual Fit

- Grafana Live is intended for streaming metrics and live telemetry
- Using it solely for error surfacing feels like a workaround rather than a first-class use case
- Tight coupling to WebSockets for error handling increases long-term maintenance risk

### Option 2: Use Grafana Dashboard Variables + Business Text Plugin (Current Approach)

Continue using dashboard-scoped variables to propagate error state and render error messages using the **Business Text** plugin.

#### How it works

- Errors are captured and written into dashboard variables
- The Business Text plugin renders error messages conditionally based on variable values
- Error visibility is scoped naturally to the dashboard instance and browser tab

#### Pros

- No WebSocket or proxy configuration required
- Errors are correctly scoped per dashboard instance
- Works reliably in multi-user and multi-tab scenarios
- Uses standard Grafana features with predictable behavior
- Easy to reason about and debug

#### Cons

- Implementation is admittedly “dirty” and non-idiomatic
- Dashboard variables are not designed primarily for error transport
- Business Text plugin is used in a way that stretches its intended purpose
- Accumulates technical debt over time

## Decision Outcome

We will **continue using Grafana dashboard variables combined with the Business Text plugin** for error surfacing.

While imperfect, this approach provides correct scoping, predictable behavior, and low operational overhead. In contrast, Grafana Live introduces significant configuration risks and architectural mismatches for this specific problem, particularly around instance-level error isolation.

At this time, the trade-offs of adopting Grafana Live outweigh its benefits.

## Consequences

### Positive Consequences

- Correct error scoping per dashboard instance and user
- No additional infrastructure or WebSocket-related risks
- Minimal operational complexity
- Known behavior and failure modes

### Negative Consequences

- Continued reliance on a non-ideal, somewhat hacky solution
- Increased technical debt
- Error handling logic remains distributed across dashboard configuration and plugins

### Neutral Consequences

- No changes to deployment or networking architecture
- No changes to plugin backend/frontend responsibilities
- Decision can be revisited if Grafana introduces better instance-scoped Live semantics

## Summary

Although Grafana Live is a powerful real-time feature, it is not a good fit for error surfacing that requires strict dashboard-instance isolation. The current approach—using dashboard variables and the Business Text plugin—remains the most pragmatic solution despite its imperfections. The decision prioritizes correctness, simplicity, and operational safety over architectural elegance.