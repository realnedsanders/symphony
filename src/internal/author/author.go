// Package author is the goal-state authoring operation shared by the web UI
// and the agent-callable interface.
package author

import (
	"context"

	"github.com/realnedsanders/symphony/src/internal/model"
	"github.com/realnedsanders/symphony/src/internal/store"
)

// Apply stores goal as SysML v2 in git and returns the commit id.
func Apply(ctx context.Context, dir string, goal model.Goal) (string, error) {
	return store.WriteGoal(ctx, dir, goal, "author goal-state")
}
