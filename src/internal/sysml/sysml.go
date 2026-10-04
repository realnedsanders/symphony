// Package sysml emits a SysML v2 textual subset and reads it back with OpenSysML.
package sysml

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
	"symphony/internal/model"
)

const (
	goalPackage     = "SymphonyGoal"
	collabPackage   = "SymphonyCollab"
	ceremonyPackage = "SymphonyCeremonies"
	partDefName     = "Component"
	sensorDefName   = "SensorContract"
)

var ident = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Node is one SysML element in the subset this toolchain stores.
type Node struct {
	Kind     string // partDef, portDef, partUsage, portUsage, requirementUsage
	Name     string
	Type     string
	Attrs    map[string]string
	Children []Node
}

var (
	clientOnce sync.Once
	client     opensysml.Client
	clientErr  error
)

func parser(ctx context.Context) (opensysml.Client, error) {
	clientOnce.Do(func() {
		client, clientErr = opensysml.New()
	})
	if clientErr != nil {
		return nil, clientErr
	}
	return client, nil
}

// EmitPackage renders nodes as a SysML v2 package.
func EmitPackage(name string, nodes []Node) (string, error) {
	if !ident.MatchString(name) {
		return "", fmt.Errorf("sysml: package name %q is not an identifier", name)
	}
	var b strings.Builder
	b.WriteString("package ")
	b.WriteString(name)
	b.WriteString(" {\n")
	for _, node := range nodes {
		if err := writeNode(&b, node, 1); err != nil {
			return "", err
		}
	}
	b.WriteString("}\n")
	return b.String(), nil
}

func writeNode(b *strings.Builder, node Node, depth int) error {
	if !ident.MatchString(node.Name) {
		return fmt.Errorf("sysml: name %q is not an identifier", node.Name)
	}
	pad := strings.Repeat("    ", depth)
	switch node.Kind {
	case "partDef":
		fmt.Fprintf(b, "%spart def %s;\n", pad, node.Name)
		return nil
	case "portDef":
		fmt.Fprintf(b, "%sport def %s {\n", pad, node.Name)
	case "partUsage":
		if node.Type != "" {
			fmt.Fprintf(b, "%spart %s : %s {\n", pad, node.Name, node.Type)
		} else {
			fmt.Fprintf(b, "%spart %s {\n", pad, node.Name)
		}
	case "portUsage":
		if len(node.Attrs) == 0 && len(node.Children) == 0 {
			fmt.Fprintf(b, "%sport %s : %s;\n", pad, node.Name, node.Type)
			return nil
		}
		fmt.Fprintf(b, "%sport %s : %s {\n", pad, node.Name, node.Type)
	case "requirementUsage":
		fmt.Fprintf(b, "%srequirement %s {\n", pad, node.Name)
	default:
		return fmt.Errorf("sysml: unknown kind %q", node.Kind)
	}
	keys := make([]string, 0, len(node.Attrs))
	for key := range node.Attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !ident.MatchString(key) {
			return fmt.Errorf("sysml: attribute %q is not an identifier", key)
		}
		quoted, err := quote(node.Attrs[key])
		if err != nil {
			return err
		}
		fmt.Fprintf(b, "%s    attribute %s = %s;\n", pad, key, quoted)
	}
	for _, child := range node.Children {
		if err := writeNode(b, child, depth+1); err != nil {
			return err
		}
	}
	fmt.Fprintf(b, "%s}\n", pad)
	return nil
}

func quote(value string) (string, error) {
	if strings.Contains(value, "\n") {
		return "", fmt.Errorf("sysml: attribute values cannot contain newlines")
	}
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`, nil
}

// ParseFile parses a SysML v2 file and returns the package members.
func ParseFile(ctx context.Context, path string) ([]Node, error) {
	c, err := parser(ctx)
	if err != nil {
		return nil, err
	}
	parsed, err := c.ParseFile(ctx, path)
	if err != nil {
		return nil, err
	}
	return nodesFromModel(ctx, c, parsed)
}

// ParseSource parses SysML v2 text.
func ParseSource(ctx context.Context, content string) ([]Node, error) {
	c, err := parser(ctx)
	if err != nil {
		return nil, err
	}
	parsed, err := c.ParseSource(ctx, content)
	if err != nil {
		return nil, err
	}
	return nodesFromModel(ctx, c, parsed)
}

func nodesFromModel(ctx context.Context, c opensysml.Client, parsed *opensysml.Model) ([]Node, error) {
	if parsed == nil || !parsed.OK() {
		return nil, fmt.Errorf("sysml: model has errors: %v", diagnostics(parsed))
	}
	pkg := packageSymbol(parsed)
	if pkg == nil {
		return nil, fmt.Errorf("sysml: model has no package")
	}
	full, err := c.LookupSymbol(ctx, parsed, pkg.ID)
	if err != nil {
		return nil, err
	}
	var nodes []Node
	for _, id := range full.ChildIDs {
		node, ok, err := loadNode(ctx, c, parsed, id)
		if err != nil {
			return nil, err
		}
		if ok {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func packageSymbol(parsed *opensysml.Model) *opensysml.Symbol {
	roots := parsed.Roots
	if len(roots) == 0 && parsed.Root != nil {
		roots = []*opensysml.Symbol{parsed.Root}
	}
	for _, root := range roots {
		if root == nil {
			continue
		}
		if root.Kind == "package" && root.ID != "" {
			return root
		}
		for _, id := range root.ChildIDs {
			if id != "" && !strings.Contains(id, "::") {
				return &opensysml.Symbol{ID: id, Name: id, Kind: "package"}
			}
		}
	}
	return nil
}

func loadNode(ctx context.Context, c opensysml.Client, parsed *opensysml.Model, id string) (Node, bool, error) {
	if id == "" || strings.HasSuffix(id, "::") {
		return Node{}, false, nil
	}
	sym, err := c.LookupSymbol(ctx, parsed, id)
	if err != nil {
		return Node{}, false, nil
	}
	switch sym.Kind {
	case "partDef", "portDef", "partUsage", "portUsage", "requirementUsage":
	default:
		return Node{}, false, nil
	}
	node := Node{
		Kind:  sym.Kind,
		Name:  sym.Name,
		Attrs: attrsOf(sym),
	}
	if sym.Type != nil {
		node.Type = sym.Type.Declared
	}
	for _, childID := range sym.ChildIDs {
		child, ok, err := loadNode(ctx, c, parsed, childID)
		if err != nil {
			return Node{}, false, err
		}
		if ok && child.Kind == "portUsage" {
			node.Children = append(node.Children, child)
		}
	}
	return node, true, nil
}

func attrsOf(sym *opensysml.Symbol) map[string]string {
	if len(sym.Attributes) == 0 {
		return nil
	}
	attrs := make(map[string]string, len(sym.Attributes))
	for _, attr := range sym.Attributes {
		text, ok := stringValue(attr.Value)
		if !ok {
			continue
		}
		attrs[attr.Name] = text
	}
	if len(attrs) == 0 {
		return nil
	}
	return attrs
}

func stringValue(value opensysml.Value) (string, bool) {
	text, ok := value.(opensysml.String)
	if !ok || value == nil {
		return "", false
	}
	return string(text), true
}

func diagnostics(parsed *opensysml.Model) string {
	if parsed == nil {
		return "nil model"
	}
	parts := make([]string, 0, len(parsed.Diagnostics))
	for _, d := range parsed.Diagnostics {
		parts = append(parts, d.String())
	}
	return strings.Join(parts, "; ")
}

func attr(node Node, key string) string {
	if node.Attrs == nil {
		return ""
	}
	return node.Attrs[key]
}

// EmitGoal writes a goal-state package.
func EmitGoal(goal model.Goal) (string, error) {
	var nodes []Node
	if len(goal.Parts) > 0 || len(goal.Places) > 0 {
		nodes = append(nodes, Node{Kind: "partDef", Name: partDefName})
	}
	if len(goal.Sensors) > 0 {
		nodes = append(nodes, Node{Kind: "partDef", Name: sensorDefName})
	}
	for _, iface := range goal.Interfaces {
		nodes = append(nodes, Node{
			Kind:  "portDef",
			Name:  iface.Name,
			Attrs: map[string]string{"shape": iface.Shape},
		})
	}
	placesByPart := map[string][]model.Place{}
	for _, place := range goal.Places {
		placesByPart[place.Part] = append(placesByPart[place.Part], place)
	}
	configs := map[string][]model.Configuration{}
	for _, cfg := range goal.Configurations {
		configs[cfg.Part] = append(configs[cfg.Part], cfg)
	}
	owners := map[string]string{}
	for _, owner := range goal.Owners {
		owners[owner.Part] = owner.Owner
	}
	for _, part := range goal.Parts {
		node := Node{Kind: "partUsage", Name: part.Name, Type: partDefName, Attrs: map[string]string{}}
		if owner := owners[part.Name]; owner != "" {
			node.Attrs["owner"] = owner
		}
		for _, cfg := range configs[part.Name] {
			node.Attrs["config_"+cfg.Key] = cfg.Value
		}
		if len(node.Attrs) == 0 {
			node.Attrs = nil
		}
		for _, place := range placesByPart[part.Name] {
			node.Children = append(node.Children, Node{
				Kind: "portUsage",
				Name: place.Port,
				Type: place.Interface,
			})
		}
		nodes = append(nodes, node)
	}
	for _, decision := range goal.Decisions {
		nodes = append(nodes, Node{
			Kind: "requirementUsage",
			Name: decision.Name,
			Attrs: map[string]string{
				"about":       decision.About,
				"choice":      decision.Choice,
				"alternative": decision.Alternative,
				"rationale":   decision.Rationale,
				"status":      decision.Status,
			},
		})
	}
	for _, sensor := range goal.Sensors {
		nodes = append(nodes, Node{
			Kind: "partUsage",
			Name: sensor.Name,
			Type: sensorDefName,
			Attrs: map[string]string{
				"observes": sensor.Observes,
				"aim":      sensor.Aim,
				"outside":  sensor.Outside,
			},
		})
	}
	return EmitPackage(goalPackage, nodes)
}

// ParseGoal interprets a goal-state package.
func ParseGoal(nodes []Node) model.Goal {
	var goal model.Goal
	for _, node := range nodes {
		switch {
		case node.Kind == "portDef":
			goal.Interfaces = append(goal.Interfaces, model.Interface{Name: node.Name, Shape: attr(node, "shape")})
		case node.Kind == "requirementUsage":
			goal.Decisions = append(goal.Decisions, model.Decision{
				Name:        node.Name,
				About:       attr(node, "about"),
				Choice:      attr(node, "choice"),
				Alternative: attr(node, "alternative"),
				Rationale:   attr(node, "rationale"),
				Status:      attr(node, "status"),
			})
		case node.Kind == "partUsage" && node.Type == sensorDefName:
			goal.Sensors = append(goal.Sensors, model.SensorContract{
				Name:     node.Name,
				Observes: attr(node, "observes"),
				Aim:      attr(node, "aim"),
				Outside:  attr(node, "outside"),
			})
		case node.Kind == "partUsage":
			def := node.Type
			if def == "" {
				def = partDefName
			}
			goal.Parts = append(goal.Parts, model.Part{Name: node.Name, Def: def})
			if owner := attr(node, "owner"); owner != "" {
				goal.Owners = append(goal.Owners, model.Ownership{Part: node.Name, Owner: owner})
			}
			for key, value := range node.Attrs {
				if rest, ok := strings.CutPrefix(key, "config_"); ok {
					goal.Configurations = append(goal.Configurations, model.Configuration{Part: node.Name, Key: rest, Value: value})
				}
			}
			sort.Slice(goal.Configurations, func(i, j int) bool {
				if goal.Configurations[i].Part != goal.Configurations[j].Part {
					return goal.Configurations[i].Part < goal.Configurations[j].Part
				}
				return goal.Configurations[i].Key < goal.Configurations[j].Key
			})
			for _, child := range node.Children {
				if child.Kind != "portUsage" {
					continue
				}
				goal.Places = append(goal.Places, model.Place{Part: node.Name, Port: child.Name, Interface: child.Type})
			}
		}
	}
	return goal
}

// ParseGoalFile reads a goal-state file with the SysML v2 parser.
func ParseGoalFile(ctx context.Context, path string) (model.Goal, error) {
	nodes, err := ParseFile(ctx, path)
	if err != nil {
		return model.Goal{}, err
	}
	return ParseGoal(nodes), nil
}

func numbered(prefix string, values []string) map[string]string {
	attrs := make(map[string]string, len(values))
	for i, value := range values {
		attrs[fmt.Sprintf("%s_%d", prefix, i)] = value
	}
	return attrs
}

func readNumbered(attrs map[string]string, prefix string) []string {
	type item struct {
		n int
		v string
	}
	var items []item
	needle := prefix + "_"
	for key, value := range attrs {
		if !strings.HasPrefix(key, needle) {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(key[len(needle):], "%d", &n); err != nil {
			continue
		}
		items = append(items, item{n: n, v: value})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].n < items[j].n })
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.v
	}
	return out
}

func merge(maps ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, m := range maps {
		for key, value := range m {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// EmitCollab writes collaboration records.
func EmitCollab(collab model.Collab) (string, error) {
	var nodes []Node
	for _, gap := range collab.Gaps {
		nodes = append(nodes, Node{
			Kind: "partUsage",
			Name: gap.ID,
			Attrs: map[string]string{
				"recordKind": "gap",
				"element":    gap.Subject,
				"summary":    gap.Summary,
				"priority":   fmt.Sprintf("%d", gap.Priority),
				"parent":     gap.ParentID,
			},
		})
	}
	for _, proposal := range collab.Proposals {
		nodes = append(nodes, Node{
			Kind: "partUsage",
			Name: proposal.ID,
			Attrs: merge(map[string]string{
				"recordKind": "proposal",
				"gap":        proposal.GapID,
				"branch":     proposal.Branch,
				"worktree":   proposal.Worktree,
				"phase":      proposal.State,
			}, numbered("touch", proposal.Touches)),
		})
	}
	for _, conflict := range collab.Conflicts {
		nodes = append(nodes, Node{
			Kind: "partUsage",
			Name: conflict.ID,
			Attrs: merge(map[string]string{
				"recordKind": "conflict",
				"phase":      conflict.State,
				"resolution": conflict.Resolution,
				"preferred":  conflict.Preferred,
			}, numbered("proposal", conflict.ProposalIDs), numbered("element", conflict.Elements), numbered("rationale", conflict.Rationales), numbered("reading", conflict.ReadingIDs)),
		})
	}
	return EmitPackage(collabPackage, nodes)
}

// ParseCollab interprets collaboration records.
func ParseCollab(nodes []Node) model.Collab {
	var collab model.Collab
	for _, node := range nodes {
		if node.Kind != "partUsage" {
			continue
		}
		switch attr(node, "recordKind") {
		case "gap":
			var priority int
			if raw := attr(node, "priority"); raw != "" {
				if _, err := fmt.Sscanf(raw, "%d", &priority); err != nil {
					priority = 0
				}
			}
			collab.Gaps = append(collab.Gaps, model.WorkGap{
				ID:       node.Name,
				Subject:  attr(node, "element"),
				Summary:  attr(node, "summary"),
				Priority: priority,
				ParentID: attr(node, "parent"),
			})
		case "proposal":
			collab.Proposals = append(collab.Proposals, model.Proposal{
				ID:       node.Name,
				GapID:    attr(node, "gap"),
				Branch:   attr(node, "branch"),
				Worktree: attr(node, "worktree"),
				State:    attr(node, "phase"),
				Touches:  readNumbered(node.Attrs, "touch"),
			})
		case "conflict":
			collab.Conflicts = append(collab.Conflicts, model.Conflict{
				ID:          node.Name,
				ProposalIDs: readNumbered(node.Attrs, "proposal"),
				Elements:    readNumbered(node.Attrs, "element"),
				Rationales:  readNumbered(node.Attrs, "rationale"),
				ReadingIDs:  readNumbered(node.Attrs, "reading"),
				State:       attr(node, "phase"),
				Resolution:  attr(node, "resolution"),
				Preferred:   attr(node, "preferred"),
			})
		}
	}
	return collab
}

// EmitCeremonies writes standup and retro records.
func EmitCeremonies(records model.Ceremonies) (string, error) {
	var nodes []Node
	for _, standup := range records.Standups {
		nodes = append(nodes, Node{
			Kind: "partUsage",
			Name: standup.ID,
			Attrs: merge(map[string]string{
				"recordKind": "standup",
				"at":         standup.At,
				"since":      standup.Since,
			}, numbered("movement", standup.CurrentMovement), numbered("proposalMovement", standup.ProposalMovement), numbered("blocker", standup.Blockers)),
		})
	}
	for _, retro := range records.Retros {
		attrs := merge(map[string]string{
			"recordKind": "retro",
			"at":         retro.At,
		}, numbered("shipped", retro.Shipped))
		for i, rejected := range retro.Rejected {
			attrs[fmt.Sprintf("rejected_%d", i)] = rejected.Alternative
			attrs[fmt.Sprintf("why_%d", i)] = rejected.Rationale
		}
		nodes = append(nodes, Node{Kind: "partUsage", Name: retro.ID, Attrs: attrs})
	}
	return EmitPackage(ceremonyPackage, nodes)
}

// ParseCeremonies interprets ceremony records.
func ParseCeremonies(nodes []Node) model.Ceremonies {
	var records model.Ceremonies
	for _, node := range nodes {
		if node.Kind != "partUsage" {
			continue
		}
		switch attr(node, "recordKind") {
		case "standup":
			records.Standups = append(records.Standups, model.Standup{
				ID:               node.Name,
				At:               attr(node, "at"),
				Since:            attr(node, "since"),
				CurrentMovement:  readNumbered(node.Attrs, "movement"),
				ProposalMovement: readNumbered(node.Attrs, "proposalMovement"),
				Blockers:         readNumbered(node.Attrs, "blocker"),
			})
		case "retro":
			alts := readNumbered(node.Attrs, "rejected")
			whys := readNumbered(node.Attrs, "why")
			retro := model.Retro{
				ID:      node.Name,
				At:      attr(node, "at"),
				Shipped: readNumbered(node.Attrs, "shipped"),
			}
			for i, alt := range alts {
				why := ""
				if i < len(whys) {
					why = whys[i]
				}
				retro.Rejected = append(retro.Rejected, model.Rejection{Alternative: alt, Rationale: why})
			}
			records.Retros = append(records.Retros, retro)
		}
	}
	return records
}
