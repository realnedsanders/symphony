package diff

import (
	"sort"
	"strings"

	"github.com/realnedsanders/symphony/src/internal/model"
)

// Compare diffs goal against sensor readings and open proposals.
// A stale or silent reading is reported on its own and is not a drift.
// A property outside a fresh reading's coverage is a gap, not a contradiction.
func Compare(goal model.Goal, readings []model.Reading, proposals []model.Proposal) model.Report {
	fresh := map[string][]model.Reading{}
	var report model.Report
	for _, reading := range readings {
		switch reading.Freshness {
		case model.Stale:
			report.Stale = append(report.Stale, reading.ID)
		case model.Silent:
			report.Silent = append(report.Silent, reading.ID)
		default:
			fresh[reading.Subject] = append(fresh[reading.Subject], reading)
		}
	}
	for subject, group := range fresh {
		sort.Slice(group, func(i, j int) bool { return group[i].ID < group[j].ID })
		fresh[subject] = group
	}
	sort.Strings(report.Stale)
	sort.Strings(report.Silent)

	coveredSubject := func(name string) bool {
		if _, ok := fresh[name]; ok {
			return true
		}
		prefix := name + "."
		for subject := range fresh {
			if strings.HasPrefix(subject, prefix) {
				return true
			}
		}
		return false
	}

	for _, part := range goal.Parts {
		if !coveredSubject(part.Name) {
			report.Gaps = append(report.Gaps, model.Gap{
				ID:      "part:" + part.Name,
				Subject: part.Name,
				Summary: "no observation",
			})
		}
	}
	for _, cfg := range goal.Configurations {
		// A nested subject such as ingest.input does not observe the part's
		// own configuration. Only readings of the part itself can cover it,
		// and each of those readings is kept: one sensor agreeing does not
		// erase another sensor's contradiction.
		covered := false
		for _, reading := range fresh[cfg.Part] {
			if !reading.Covers(cfg.Key) {
				continue
			}
			covered = true
			observed := reading.Observation[cfg.Key]
			if observed != cfg.Value {
				report.Drifts = append(report.Drifts, model.Drift{
					Subject:   cfg.Part,
					Property:  cfg.Key,
					Goal:      cfg.Value,
					Observed:  observed,
					ReadingID: reading.ID,
				})
			}
		}
		if !covered {
			report.Gaps = append(report.Gaps, model.Gap{
				ID:       "config:" + cfg.Part + ":" + cfg.Key,
				Subject:  cfg.Part,
				Property: cfg.Key,
				Summary:  "missing " + cfg.Key,
			})
		}
	}

	for _, iface := range goal.Interfaces {
		impact := model.Impact{Interface: iface.Name, Shape: iface.Shape}
		for _, place := range goal.Places {
			if place.Interface != iface.Name {
				continue
			}
			placeImpact := model.ImpactPlace{ID: place.ID(), Proposals: proposalsTouching(proposals, place.ID(), iface.Name)}
			var matched string
			sawMatch := false
			for _, reading := range fresh[place.ID()] {
				if !reading.Covers("shape") {
					continue
				}
				observed := reading.Observation["shape"]
				if observed != iface.Shape {
					if !placeImpact.SeesOldShape {
						placeImpact.ObservedShape = observed
						placeImpact.SeesOldShape = true
					}
					continue
				}
				if !sawMatch {
					matched = observed
					sawMatch = true
				}
			}
			if !placeImpact.SeesOldShape && sawMatch {
				placeImpact.ObservedShape = matched
			}
			impact.Places = append(impact.Places, placeImpact)
		}
		if len(impact.Places) > 0 {
			report.Impacts = append(report.Impacts, impact)
		}
	}

	for _, sensor := range goal.Sensors {
		group := fresh[sensor.Name]
		if len(group) == 0 {
			report.Gaps = append(report.Gaps, model.Gap{
				ID:      "sensor:" + sensor.Name,
				Subject: sensor.Name,
				Summary: "no observation of sensor contract",
			})
			continue
		}
		for _, field := range []struct{ name, goal string }{
			{"observes", sensor.Observes},
			{"aim", sensor.Aim},
			{"outside", sensor.Outside},
		} {
			seen := map[string]bool{}
			for _, reading := range group {
				if !reading.Covers(field.name) {
					continue
				}
				observed := reading.Observation[field.name]
				if observed == field.goal || seen[observed] {
					continue
				}
				seen[observed] = true
				report.ContractChanges = append(report.ContractChanges, model.ContractChange{
					Sensor:   sensor.Name,
					Field:    field.name,
					Goal:     field.goal,
					Observed: observed,
				})
			}
		}
	}

	sort.Slice(report.Gaps, func(i, j int) bool { return report.Gaps[i].ID < report.Gaps[j].ID })
	sort.Slice(report.Drifts, func(i, j int) bool {
		if report.Drifts[i].Subject != report.Drifts[j].Subject {
			return report.Drifts[i].Subject < report.Drifts[j].Subject
		}
		if report.Drifts[i].Property != report.Drifts[j].Property {
			return report.Drifts[i].Property < report.Drifts[j].Property
		}
		return report.Drifts[i].ReadingID < report.Drifts[j].ReadingID
	})
	sort.Slice(report.ContractChanges, func(i, j int) bool {
		if report.ContractChanges[i].Sensor != report.ContractChanges[j].Sensor {
			return report.ContractChanges[i].Sensor < report.ContractChanges[j].Sensor
		}
		if report.ContractChanges[i].Field != report.ContractChanges[j].Field {
			return report.ContractChanges[i].Field < report.ContractChanges[j].Field
		}
		return report.ContractChanges[i].Observed < report.ContractChanges[j].Observed
	})
	return report
}

func proposalsTouching(proposals []model.Proposal, placeID, iface string) []string {
	var ids []string
	for _, proposal := range proposals {
		if proposal.State != "open" && proposal.State != "preferred" {
			continue
		}
		for _, touch := range proposal.Touches {
			if touch == placeID || touch == iface {
				ids = append(ids, proposal.ID)
				break
			}
		}
	}
	sort.Strings(ids)
	return ids
}

// Format renders a report as stable text.
func Format(report model.Report) string {
	var b strings.Builder
	b.WriteString("gaps:\n")
	if len(report.Gaps) == 0 {
		b.WriteString("- none\n")
	}
	for _, gap := range report.Gaps {
		if gap.Property != "" {
			b.WriteString("- " + gap.Subject + " " + gap.Property + " " + gap.Summary + "\n")
		} else {
			b.WriteString("- " + gap.Subject + " " + gap.Summary + "\n")
		}
	}
	b.WriteString("drifts:\n")
	if len(report.Drifts) == 0 {
		b.WriteString("- none\n")
	}
	for _, drift := range report.Drifts {
		b.WriteString("- " + drift.Subject + " " + drift.Property + " goal=" + drift.Goal + " observed=" + drift.Observed + " reading=" + drift.ReadingID + "\n")
	}
	b.WriteString("impacts:\n")
	if len(report.Impacts) == 0 {
		b.WriteString("- none\n")
	}
	for _, impact := range report.Impacts {
		b.WriteString("- " + impact.Interface + " shape=" + impact.Shape + "\n")
		for _, place := range impact.Places {
			old := "false"
			if place.SeesOldShape {
				old = "true"
			}
			b.WriteString("  - " + place.ID + " observed=" + place.ObservedShape + " old=" + old + " proposals=" + strings.Join(place.Proposals, ",") + "\n")
		}
	}
	b.WriteString("contracts:\n")
	if len(report.ContractChanges) == 0 {
		b.WriteString("- none\n")
	}
	for _, change := range report.ContractChanges {
		b.WriteString("- " + change.Sensor + " " + change.Field + " goal=" + change.Goal + " observed=" + change.Observed + "\n")
	}
	b.WriteString("stale:\n")
	if len(report.Stale) == 0 {
		b.WriteString("- none\n")
	}
	for _, id := range report.Stale {
		b.WriteString("- " + id + "\n")
	}
	b.WriteString("silent:\n")
	if len(report.Silent) == 0 {
		b.WriteString("- none\n")
	}
	for _, id := range report.Silent {
		b.WriteString("- " + id + "\n")
	}
	return b.String()
}
