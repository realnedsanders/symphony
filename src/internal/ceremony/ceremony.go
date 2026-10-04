// Package ceremony accepts and rejects proposals and records standup and retro.
package ceremony

import (
	"context"
	"fmt"
	"time"

	"github.com/realnedsanders/symphony/src/internal/gitx"
	"github.com/realnedsanders/symphony/src/internal/model"
	"github.com/realnedsanders/symphony/src/internal/store"
)

// Accept merges a proposal branch into the goal-state and marks it accepted.
func Accept(ctx context.Context, dir, proposalID string) (string, error) {
	collab, err := store.ReadCollab(ctx, dir)
	if err != nil {
		return "", err
	}
	index := -1
	for i, proposal := range collab.Proposals {
		if proposal.ID == proposalID {
			index = i
			break
		}
	}
	if index < 0 {
		return "", fmt.Errorf("ceremony: proposal %s not found", proposalID)
	}
	if _, err := gitx.Run(dir, "merge", "--no-edit", collab.Proposals[index].Branch); err != nil {
		return "", err
	}
	collab, err = store.ReadCollab(ctx, dir)
	if err != nil {
		return "", err
	}
	for i, proposal := range collab.Proposals {
		if proposal.ID == proposalID {
			collab.Proposals[i].State = "accepted"
			break
		}
	}
	return store.WriteCollab(dir, collab, "accept "+proposalID)
}

// Reject leaves the proposal unmerged and records the alternative on the element.
func Reject(ctx context.Context, dir, proposalID, element, alternative, rationale string) (string, error) {
	collab, err := store.ReadCollab(ctx, dir)
	if err != nil {
		return "", err
	}
	found := false
	for i, proposal := range collab.Proposals {
		if proposal.ID == proposalID {
			collab.Proposals[i].State = "rejected"
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("ceremony: proposal %s not found", proposalID)
	}
	goal, err := store.ReadGoal(ctx, dir)
	if err != nil {
		return "", err
	}
	decision := model.Decision{
		Name:        "reject" + proposalID,
		About:       element,
		Choice:      "rejected",
		Alternative: alternative,
		Rationale:   rationale,
		Status:      "rejected",
	}
	replaced := false
	for i, existing := range goal.Decisions {
		if existing.Name == decision.Name {
			goal.Decisions[i] = decision
			replaced = true
			break
		}
	}
	if !replaced {
		goal.Decisions = append(goal.Decisions, decision)
	}
	if _, err := store.WriteGoal(ctx, dir, goal, "reject "+proposalID); err != nil {
		return "", err
	}
	return store.WriteCollab(dir, collab, "reject "+proposalID+" proposal")
}

// RecordStandup derives movement since a point and writes the standup record.
func RecordStandup(ctx context.Context, dir, id string, since time.Time, readings []model.Reading, blockers []string) (model.Standup, error) {
	collab, err := store.ReadCollab(ctx, dir)
	if err != nil {
		return model.Standup{}, err
	}
	standup := model.Standup{
		ID:       id,
		At:       time.Now().UTC().Format(time.RFC3339),
		Since:    since.UTC().Format(time.RFC3339),
		Blockers: append([]string{}, blockers...),
	}
	for _, reading := range readings {
		if reading.Timestamp.Before(since) {
			continue
		}
		standup.CurrentMovement = append(standup.CurrentMovement, reading.ID+" "+reading.Subject+" "+string(reading.Freshness))
	}
	for _, proposal := range collab.Proposals {
		switch proposal.State {
		case "accepted", "rejected":
			continue
		default:
			standup.ProposalMovement = append(standup.ProposalMovement, proposal.ID+" "+proposal.State)
		}
	}
	for _, conflict := range collab.Conflicts {
		if conflict.State == "open" {
			standup.Blockers = append(standup.Blockers, "conflict "+conflict.ID)
		}
	}
	records, err := store.ReadCeremonies(ctx, dir)
	if err != nil {
		return model.Standup{}, err
	}
	replaced := false
	for i, existing := range records.Standups {
		if existing.ID == standup.ID {
			records.Standups[i] = standup
			replaced = true
			break
		}
	}
	if !replaced {
		records.Standups = append(records.Standups, standup)
	}
	if _, err := store.WriteCeremonies(dir, records, "standup "+id); err != nil {
		return model.Standup{}, err
	}
	reloaded, err := store.ReadCeremonies(ctx, dir)
	if err != nil {
		return model.Standup{}, err
	}
	for _, existing := range reloaded.Standups {
		if existing.ID == id {
			return existing, nil
		}
	}
	return model.Standup{}, fmt.Errorf("ceremony: standup %s missing after write", id)
}

// RecordRetro writes what has shipped and what was rejected, from the store.
func RecordRetro(ctx context.Context, dir, id string) (model.Retro, error) {
	collab, err := store.ReadCollab(ctx, dir)
	if err != nil {
		return model.Retro{}, err
	}
	goal, err := store.ReadGoal(ctx, dir)
	if err != nil {
		return model.Retro{}, err
	}
	retro := model.Retro{ID: id, At: time.Now().UTC().Format(time.RFC3339)}
	for _, proposal := range collab.Proposals {
		if proposal.State == "accepted" {
			retro.Shipped = append(retro.Shipped, proposal.ID)
		}
	}
	for _, decision := range goal.Decisions {
		if decision.Status == "rejected" {
			retro.Rejected = append(retro.Rejected, model.Rejection{
				Alternative: decision.Alternative,
				Rationale:   decision.Rationale,
			})
		}
	}
	records, err := store.ReadCeremonies(ctx, dir)
	if err != nil {
		return model.Retro{}, err
	}
	replaced := false
	for i, existing := range records.Retros {
		if existing.ID == retro.ID {
			records.Retros[i] = retro
			replaced = true
			break
		}
	}
	if !replaced {
		records.Retros = append(records.Retros, retro)
	}
	if _, err := store.WriteCeremonies(dir, records, "retro "+id); err != nil {
		return model.Retro{}, err
	}
	reloaded, err := store.ReadCeremonies(ctx, dir)
	if err != nil {
		return model.Retro{}, err
	}
	for _, existing := range reloaded.Retros {
		if existing.ID == id {
			return existing, nil
		}
	}
	return model.Retro{}, fmt.Errorf("ceremony: retro %s missing after write", id)
}
