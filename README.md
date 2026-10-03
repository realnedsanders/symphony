# Symphony

An open toolchain for distributed multi-agent, multi-stakeholder systems development and formalization.

Symphony reduces the cognitive load of iteratively building deep technical systems across stakeholder roles, and keeps the interconnections between components explicit. People and agents share one model of the system. The work to be done is the difference between the system they are building toward and the system the sensors can see.

Design stage: this repository holds the intent of the toolchain.

## The model

Two states, one diff.

**Goal-state** is the specification stakeholders are building toward. Structure, interfaces, configuration, ownership, and decisions all live here. Chosen paths and rejected paths are both kept, with the rationale for each. The persisted form is SysML v2, held in git. Agents and the web UI are the readers and writers of that notation.

**Current-state** is the partial twin of what is actually implemented. Sensors fill it in by driving model properties from real sources. A sensor's contract and configuration are themselves elements of the model: what it observes, how it is aimed, and what lies outside its coverage. Every reading carries the observation, the time, and the coverage, so a missing fact can be told apart from a stale or silent connector.

The connector set is small and bound to stable specifications. The inventory is under [Connectors](#connectors). Modeling the contract next to the system it observes is what keeps that set manageable: a change in what a sensor claims is a change in the model, visible in the same diff as everything else.

**The diff** is the work. Goal-state against current-state yields what is unsatisfied, what has drifted, and the impact of a change. Update an interface and the model shows every place that interface is required, which of those places the sensors still see the old shape, and which open proposals already touch them.

## Collaboration

The goal-state is tracked in git, across the many repositories of the system.

- Proposals are branches.
- Agents test fitment in isolated worktrees.
- When proposals conflict, Symphony captures the clash in model terms: the elements in dispute, the rationales already recorded, and the sensor readings that bear on them. Resolution happens on that record.
- An accepted proposal updates the goal-state. A rejected alternative stays on the elements it concerns, with the rationale.

## Surfaces

The web UI is where humans work. It captures and displays the ceremonies the team already runs, and the model is what makes those views more than a board.

- **Backlog grooming.** Gaps from the diff, decomposed and prioritized, at the granularity the work needs: a capability, a component, or a single interface change.
- **Standup.** What current-state and the open proposals have done since last time, and what is blocked.
- **Retro.** What shipped, what was rejected, and why.

The same model supports views the board cannot. Slices run along axes the team chooses: component, workstream, owner, environment, capability. Diagrams export to Mermaid, PNG, and PDF. A Wardley view is available when the question is evolution and visibility.

### Agents

Agents collaborate on the goal-state, take gaps from the diff, and test proposals in worktrees. Surfaces: an API, an MCP server, and agent skills. Day to day this is the harness an engineer already runs (Grok Build, Claude Code, Codex, Pi, and others): feature implementation, gap decomposition, and fitment against the model.

### Managers

Managers see gaps grouped by workstream, what each is blocked by, and which proposals are moving. Grooming and standup are this view, backed by sensor evidence.

### Leaders

Leaders see capability coverage: which goals have no sensor, which running elements have no owner, and where goal-state and the delivered system have diverged above the level of a single workstream.

## Scope

### Tools

- SysML v2, as the internal representation
- git for state tracking
- Go, for the toolchain
- typescript for the web ui

### Connectors

What connectors we build

- Go as a language (go code -> SysML v2)
- Git, across many repositories
- Linux container images (cgroups v2 based, docker/podman/containerd/etc)
- Kubernetes manifests
- Helm charts
- [Zarf](https://github.com/zarf-dev/zarf) and [Defense Unicorns UDS](https://github.com/defenseunicorns/uds)
- Kubernetes (live cluster)

Notes on Zarf and UDS: 

> Zarf and UDS are how configuration is packaged and propagated, including into airgapped environments.
> A package keeps its shape from the description in the goal-state, through the artifact that moves, to the configuration a sensor reads where it lands.
> That preserved structure is why they sit in the core, next to git: the goal and the delivery share a form the model can follow.

## Operating loop

1. Describe the goal-state through the UI or an agent. It is stored as SysML v2 in git.
2. Declare sensor contracts and their configuration as part of that model.
3. Sensors project the connected sources into current-state. Each reading is stamped with time and coverage.
4. The diff produces gaps, drifts, and impact.
5. Grooming prioritizes and decomposes gaps. Agents and engineers take them as branches and test fitment in worktrees.
6. Conflicts are captured on the elements, rationales, and readings involved, and resolved there.
7. Accepted work updates the goal-state. Rejected alternatives remain, with rationale. Standup and retro read and write that record.

## Inspiration

[Choir](https://github.com/Weber-GeoML/Choir) is an open protocol for distributed multi-agent formalization. Symphony takes the working arrangement: a shared formal artifact, tasks derived from what is missing, contributors using their own agents, and a durable record of what was accepted and why. Sensor evidence against the model, with coverage attached, is the check on a proposal.
