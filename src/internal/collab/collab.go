// Package collab grooms gaps, takes them as branches and worktrees, and
// records conflicts so they can be resolved on that record.
package collab

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/realnedsanders/symphony/src/internal/gitx"
	"github.com/realnedsanders/symphony/src/internal/model"
	"github.com/realnedsanders/symphony/src/internal/store"
)

// AddGap records a gap on the collaboration model.
func AddGap(ctx context.Context, dir string, gap model.WorkGap) (model.Collab, error) {
	collab, err := store.ReadCollab(ctx, dir)
	if err != nil {
		return model.Collab{}, err
	}
	replaced := false
	for i, existing := range collab.Gaps {
		if existing.ID == gap.ID {
			collab.Gaps[i] = gap
			replaced = true
			break
		}
	}
	if !replaced {
		collab.Gaps = append(collab.Gaps, gap)
	}
	if _, err := store.WriteCollab(dir, collab, "groom gap "+gap.ID); err != nil {
		return model.Collab{}, err
	}
	return store.ReadCollab(ctx, dir)
}

// Prioritize sets the priority of a gap. A larger number is more urgent.
func Prioritize(ctx context.Context, dir, id string, priority int) (model.Collab, error) {
	collab, err := store.ReadCollab(ctx, dir)
	if err != nil {
		return model.Collab{}, err
	}
	found := false
	for i, gap := range collab.Gaps {
		if gap.ID == id {
			collab.Gaps[i].Priority = priority
			found = true
			break
		}
	}
	if !found {
		return model.Collab{}, fmt.Errorf("collab: gap %s not found", id)
	}
	if _, err := store.WriteCollab(dir, collab, "prioritize "+id); err != nil {
		return model.Collab{}, err
	}
	return store.ReadCollab(ctx, dir)
}

// Decompose splits a gap into child gaps.
func Decompose(ctx context.Context, dir, id string, children []model.WorkGap) (model.Collab, error) {
	collab, err := store.ReadCollab(ctx, dir)
	if err != nil {
		return model.Collab{}, err
	}
	found := false
	for _, gap := range collab.Gaps {
		if gap.ID == id {
			found = true
			break
		}
	}
	if !found {
		return model.Collab{}, fmt.Errorf("collab: gap %s not found", id)
	}
	for _, child := range children {
		child.ParentID = id
		updated := false
		for i, existing := range collab.Gaps {
			if existing.ID == child.ID {
				collab.Gaps[i] = child
				updated = true
				break
			}
		}
		if !updated {
			collab.Gaps = append(collab.Gaps, child)
		}
	}
	if _, err := store.WriteCollab(dir, collab, "decompose "+id); err != nil {
		return model.Collab{}, err
	}
	return store.ReadCollab(ctx, dir)
}

// Ordered returns gaps from highest priority to lowest.
func Ordered(gaps []model.WorkGap) []model.WorkGap {
	out := append([]model.WorkGap(nil), gaps...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Take creates a proposal branch and an isolated worktree for a gap.
func Take(ctx context.Context, dir, gapID, proposalID string) (model.Proposal, error) {
	collab, err := store.ReadCollab(ctx, dir)
	if err != nil {
		return model.Proposal{}, err
	}
	var gap model.WorkGap
	found := false
	for _, candidate := range collab.Gaps {
		if candidate.ID == gapID {
			gap = candidate
			found = true
			break
		}
	}
	if !found {
		return model.Proposal{}, fmt.Errorf("collab: gap %s not found", gapID)
	}
	branch := "proposal/" + proposalID
	worktree := filepath.Join(dir, ".worktrees", proposalID)
	if err := os.MkdirAll(filepath.Dir(worktree), 0o755); err != nil {
		return model.Proposal{}, err
	}
	if _, err := gitx.Run(dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err != nil {
		if _, err := gitx.RunAt(dir, "worktree", "add", "-b", branch, worktree, "HEAD"); err != nil {
			return model.Proposal{}, err
		}
	} else if _, err := gitx.RunAt(dir, "worktree", "add", worktree, branch); err != nil {
		return model.Proposal{}, err
	}
	proposal := model.Proposal{
		ID:       proposalID,
		GapID:    gapID,
		Branch:   branch,
		Worktree: worktree,
		State:    "open",
		Touches:  []string{gap.Subject},
	}
	replaced := false
	for i, existing := range collab.Proposals {
		if existing.ID == proposal.ID {
			collab.Proposals[i] = proposal
			replaced = true
			break
		}
	}
	if !replaced {
		collab.Proposals = append(collab.Proposals, proposal)
	}
	if _, err := store.WriteCollab(dir, collab, "take "+proposalID); err != nil {
		return model.Proposal{}, err
	}
	return proposal, nil
}

// RecordConflict stores a clash against the elements, rationales, and readings
// that bear on it.
func RecordConflict(ctx context.Context, dir string, conflict model.Conflict) (model.Conflict, error) {
	if conflict.State == "" {
		conflict.State = "open"
	}
	collab, err := store.ReadCollab(ctx, dir)
	if err != nil {
		return model.Conflict{}, err
	}
	replaced := false
	for i, existing := range collab.Conflicts {
		if existing.ID == conflict.ID {
			collab.Conflicts[i] = conflict
			replaced = true
			break
		}
	}
	if !replaced {
		collab.Conflicts = append(collab.Conflicts, conflict)
	}
	if _, err := store.WriteCollab(dir, collab, "record conflict "+conflict.ID); err != nil {
		return model.Conflict{}, err
	}
	reloaded, err := store.ReadCollab(ctx, dir)
	if err != nil {
		return model.Conflict{}, err
	}
	for _, existing := range reloaded.Conflicts {
		if existing.ID == conflict.ID {
			return existing, nil
		}
	}
	return model.Conflict{}, fmt.Errorf("collab: conflict %s missing after write", conflict.ID)
}

// Resolve records a decision on the conflict and updates the proposal states
// named by it. preferred becomes "preferred"; the other proposals become "deferred".
func Resolve(ctx context.Context, dir, conflictID, resolution, preferred string) (model.Collab, error) {
	collab, err := store.ReadCollab(ctx, dir)
	if err != nil {
		return model.Collab{}, err
	}
	var conflict *model.Conflict
	for i := range collab.Conflicts {
		if collab.Conflicts[i].ID == conflictID {
			conflict = &collab.Conflicts[i]
			break
		}
	}
	if conflict == nil {
		return model.Collab{}, fmt.Errorf("collab: conflict %s not found", conflictID)
	}
	conflict.State = "resolved"
	conflict.Resolution = resolution
	conflict.Preferred = preferred
	named := map[string]bool{}
	for _, id := range conflict.ProposalIDs {
		named[id] = true
	}
	if !named[preferred] {
		return model.Collab{}, fmt.Errorf("collab: preferred proposal %s is not in the conflict", preferred)
	}
	for i, proposal := range collab.Proposals {
		if !named[proposal.ID] {
			continue
		}
		if proposal.ID == preferred {
			collab.Proposals[i].State = "preferred"
		} else {
			collab.Proposals[i].State = "deferred"
		}
	}
	if _, err := store.WriteCollab(dir, collab, "resolve "+conflictID); err != nil {
		return model.Collab{}, err
	}
	return store.ReadCollab(ctx, dir)
}
