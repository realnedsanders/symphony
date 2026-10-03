# Symphony

An open toolchain for distributed multi-agent, multi-stakeholder systems development and formalization

---

Symphony aims to reduce the cognitive complexity of iteratively developing deep technical computer-based systems, spanning stakeholder roles and tracking deep interconnections between disparate system components.
Stakeholders (ai agents, users, engineers, managers, leaders, etc) collaboratively build system specifications (the "goal-state") in SysMLv2.
Chosen and rejected paths are captured in this model, with rationale for each.
Collaboration is enabled via tracking the goal-state in git: proposals branch like PRs, agents can test theories/fitment in isolated worktrees, and conflicting changes can be intelligently captured and resolved.
The partial twin of this is what's actually been implemented (the "current-state"), which is completely driven by "sensors": connectors that drive the state of the model through data -- properties of the actual system.
The "current-state" always has the most up-to-date information (of what's available), and can be diff'ed against the desired state to determine what must be done (or what direction must be explored) to move towards the goal-state.

## Inspiration

[github.com/Weber-GeoML/Choir](https://github.com/Weber-GeoML/Choir) (mathematics focused)

## What

- Combines inspiration from tools like GitHub Issues/Pull Requests, Linear/Jira ticket tracking, Canva/Miro, and Cameo/system modeling tools.
- Enables fast, high fidelity tracking of workstreams within a larger overall system/project, at customizable levels of granularity and along customizable axes.
- Shows exactly what and where things break under certain conditions (i.e. code dependency A was updated with a different interface, shows all areas where that interface needs changed in the system).

## How

- Create spec
- Define sensor sources (repos/connections between repos, ???)
- system generates current-state model (or empty)
- 

## Scope

### Tools

supported tools:

- Golang
- SysML v2
- zarf
- Defense Unicorns UDS
- git (many repos)

### Usage Modes

#### AI Agents

- API
- MCP Server
- Agent Skills
- ???

#### Humans

- web-based visualizations 
    - export diagrams to mermaid/PNG/PDF
    - progress tracking
    - gaps prioritization
    - Multiple views/axes for system visualization. Wardley map too?


##### Engineers

- AI agent interface (Claude Code, Codex, Pi, Prime-Agent, etc)
    - collaborate on feature implementation, gap decomposition, 

##### Managers

- ???


##### Leaders

- ???
