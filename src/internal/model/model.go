// Package model is the in-memory operating loop: goal-state, sensor readings,
// the diff, proposals, and ceremony records. SysML v2 text exists only at the
// storage boundary.
package model

import "time"

// Goal is the specification stakeholders are building toward.
type Goal struct {
	Parts          []Part           `json:"parts"`
	Interfaces     []Interface      `json:"interfaces"`
	Places         []Place          `json:"places"`
	Configurations []Configuration  `json:"configurations"`
	Owners         []Ownership      `json:"owners"`
	Decisions      []Decision       `json:"decisions"`
	Sensors        []SensorContract `json:"sensors"`
}

// Part is one structural element of the system.
type Part struct {
	Name string `json:"name"`
	Def  string `json:"def"`
}

// Interface is a port definition. Shape is the shape the goal requires.
type Interface struct {
	Name  string `json:"name"`
	Shape string `json:"shape"`
}

// Place is one port that requires an interface.
type Place struct {
	Part      string `json:"part"`
	Port      string `json:"port"`
	Interface string `json:"interface"`
}

// ID is the stable name of the place, part.port.
func (p Place) ID() string { return p.Part + "." + p.Port }

// Configuration is one configured property of a part.
type Configuration struct {
	Part  string `json:"part"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Ownership names who owns a part.
type Ownership struct {
	Part  string `json:"part"`
	Owner string `json:"owner"`
}

// Decision records a chosen path and a rejected alternative.
type Decision struct {
	Name        string `json:"name"`
	About       string `json:"about"`
	Choice      string `json:"choice"`
	Alternative string `json:"alternative"`
	Rationale   string `json:"rationale"`
	Status      string `json:"status"`
}

// SensorContract is a sensor's claim: what it observes, how it is aimed,
// and what lies outside its coverage.
type SensorContract struct {
	Name     string `json:"name"`
	Observes string `json:"observes"`
	Aim      string `json:"aim"`
	Outside  string `json:"outside"`
}

// Freshness distinguishes a usable reading from a stale or silent connector.
type Freshness string

const (
	Fresh  Freshness = "fresh"
	Stale  Freshness = "stale"
	Silent Freshness = "silent"
)

// Reading is one sensor observation.
type Reading struct {
	ID          string            `json:"id"`
	Source      string            `json:"source"`
	Subject     string            `json:"subject"`
	Observation map[string]string `json:"observation"`
	Timestamp   time.Time         `json:"timestamp"`
	Coverage    []string          `json:"coverage"`
	Freshness   Freshness         `json:"freshness"`
}

// Covers reports whether the connector claims to have observed key.
func (r Reading) Covers(key string) bool {
	for _, covered := range r.Coverage {
		if covered == key {
			return true
		}
	}
	return false
}

// Gap is an unsatisfied goal element, or a fact no fresh connector covered.
type Gap struct {
	ID       string `json:"id"`
	Subject  string `json:"subject"`
	Property string `json:"property,omitempty"`
	Summary  string `json:"summary"`
}

// Drift is a fresh, covered observation that contradicts the goal.
type Drift struct {
	Subject   string `json:"subject"`
	Property  string `json:"property"`
	Goal      string `json:"goal"`
	Observed  string `json:"observed"`
	ReadingID string `json:"readingId"`
}

// Impact is the effect of an interface shape the goal has moved to.
type Impact struct {
	Interface string        `json:"interface"`
	Shape     string        `json:"shape"`
	Places    []ImpactPlace `json:"places"`
}

// ImpactPlace is one port that requires the interface.
type ImpactPlace struct {
	ID            string   `json:"id"`
	ObservedShape string   `json:"observedShape"`
	SeesOldShape  bool     `json:"seesOldShape"`
	Proposals     []string `json:"proposals"`
}

// ContractChange is a sensor claim that differs from the observed claim.
type ContractChange struct {
	Sensor   string `json:"sensor"`
	Field    string `json:"field"`
	Goal     string `json:"goal"`
	Observed string `json:"observed"`
}

// Report is the diff of goal-state against current-state.
type Report struct {
	Gaps            []Gap            `json:"gaps"`
	Drifts          []Drift          `json:"drifts"`
	Impacts         []Impact         `json:"impacts"`
	ContractChanges []ContractChange `json:"contractChanges"`
	Stale           []string         `json:"stale"`
	Silent          []string         `json:"silent"`
}

// WorkGap is a groomable unit of work derived from the diff.
type WorkGap struct {
	ID       string `json:"id"`
	Subject  string `json:"subject"`
	Summary  string `json:"summary"`
	Priority int    `json:"priority"`
	ParentID string `json:"parentId,omitempty"`
}

// Proposal is a branch and worktree taken for a gap.
type Proposal struct {
	ID       string   `json:"id"`
	GapID    string   `json:"gapId"`
	Branch   string   `json:"branch"`
	Worktree string   `json:"worktree"`
	State    string   `json:"state"`
	Touches  []string `json:"touches"`
}

// Conflict is a clash between proposals, recorded so it can be resolved.
type Conflict struct {
	ID          string   `json:"id"`
	ProposalIDs []string `json:"proposalIds"`
	Elements    []string `json:"elements"`
	Rationales  []string `json:"rationales"`
	ReadingIDs  []string `json:"readingIds"`
	State       string   `json:"state"`
	Resolution  string   `json:"resolution"`
	Preferred   string   `json:"preferred"`
}

// Collab is the groomed backlog, proposals, and conflicts.
type Collab struct {
	Gaps      []WorkGap  `json:"gaps"`
	Proposals []Proposal `json:"proposals"`
	Conflicts []Conflict `json:"conflicts"`
}

// Standup is one standup record.
type Standup struct {
	ID               string   `json:"id"`
	At               string   `json:"at"`
	Since            string   `json:"since"`
	CurrentMovement  []string `json:"currentMovement"`
	ProposalMovement []string `json:"proposalMovement"`
	Blockers         []string `json:"blockers"`
}

// Rejection is one rejected alternative and why.
type Rejection struct {
	Alternative string `json:"alternative"`
	Rationale   string `json:"rationale"`
}

// Retro is one retrospective record.
type Retro struct {
	ID       string      `json:"id"`
	At       string      `json:"at"`
	Shipped  []string    `json:"shipped"`
	Rejected []Rejection `json:"rejected"`
}

// Ceremonies holds standup and retro records.
type Ceremonies struct {
	Standups []Standup `json:"standups"`
	Retros   []Retro   `json:"retros"`
}
