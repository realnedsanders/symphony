// Package store persists the operating loop as SysML v2 files in git.
package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/realnedsanders/symphony/internal/gitx"
	"github.com/realnedsanders/symphony/internal/model"
	"github.com/realnedsanders/symphony/internal/sysml"
)

const (
	GoalPath     = "model/goal.sysml"
	CollabPath   = "model/collab.sysml"
	CeremonyPath = "model/ceremonies.sysml"
)

// Open ensures dir is a git repository and returns it.
func Open(dir string) (string, error) {
	if err := gitx.EnsureRepo(dir); err != nil {
		return "", err
	}
	return dir, nil
}

// Commit writes paths into git. An unchanged tree returns the current HEAD.
func Commit(dir, message string, paths ...string) (string, error) {
	if _, err := Open(dir); err != nil {
		return "", err
	}
	for _, path := range paths {
		if _, err := gitx.Run(dir, "add", "--", path); err != nil {
			return "", err
		}
	}
	if _, err := gitx.Run(dir, "diff", "--cached", "--quiet"); err == nil {
		return gitx.Run(dir, "rev-parse", "HEAD")
	}
	if _, err := gitx.Run(dir, "commit", "-m", message); err != nil {
		return "", err
	}
	return gitx.Run(dir, "rev-parse", "HEAD")
}

func write(dir, rel, content string) error {
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func read(dir, rel string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteGoal emits the goal and commits it.
func WriteGoal(ctx context.Context, dir string, goal model.Goal, message string) (string, error) {
	text, err := sysml.EmitGoal(goal)
	if err != nil {
		return "", err
	}
	if err := write(dir, GoalPath, text); err != nil {
		return "", err
	}
	return Commit(dir, message, GoalPath)
}

// ReadGoal parses the committed goal. A missing file is an empty goal.
func ReadGoal(ctx context.Context, dir string) (model.Goal, error) {
	if _, err := os.Stat(filepath.Join(dir, GoalPath)); errors.Is(err, os.ErrNotExist) {
		return model.Goal{}, nil
	} else if err != nil {
		return model.Goal{}, err
	}
	return sysml.ParseGoalFile(ctx, filepath.Join(dir, GoalPath))
}

// WriteCollab emits collaboration records and commits them.
func WriteCollab(dir string, collab model.Collab, message string) (string, error) {
	text, err := sysml.EmitCollab(collab)
	if err != nil {
		return "", err
	}
	if err := write(dir, CollabPath, text); err != nil {
		return "", err
	}
	return Commit(dir, message, CollabPath)
}

// ReadCollab parses collaboration records. A missing file is empty.
func ReadCollab(ctx context.Context, dir string) (model.Collab, error) {
	if _, err := os.Stat(filepath.Join(dir, CollabPath)); errors.Is(err, os.ErrNotExist) {
		return model.Collab{}, nil
	} else if err != nil {
		return model.Collab{}, err
	}
	text, err := read(dir, CollabPath)
	if err != nil {
		return model.Collab{}, err
	}
	nodes, err := sysml.ParseSource(ctx, text)
	if err != nil {
		return model.Collab{}, err
	}
	return sysml.ParseCollab(nodes), nil
}

// WriteCeremonies emits ceremony records and commits them.
func WriteCeremonies(dir string, records model.Ceremonies, message string) (string, error) {
	text, err := sysml.EmitCeremonies(records)
	if err != nil {
		return "", err
	}
	if err := write(dir, CeremonyPath, text); err != nil {
		return "", err
	}
	return Commit(dir, message, CeremonyPath)
}

// ReadCeremonies parses ceremony records. A missing file is empty.
func ReadCeremonies(ctx context.Context, dir string) (model.Ceremonies, error) {
	if _, err := os.Stat(filepath.Join(dir, CeremonyPath)); errors.Is(err, os.ErrNotExist) {
		return model.Ceremonies{}, nil
	} else if err != nil {
		return model.Ceremonies{}, err
	}
	text, err := read(dir, CeremonyPath)
	if err != nil {
		return model.Ceremonies{}, err
	}
	nodes, err := sysml.ParseSource(ctx, text)
	if err != nil {
		return model.Ceremonies{}, err
	}
	return sysml.ParseCeremonies(nodes), nil
}

// Head returns the current commit, or an error when the repository has none.
func Head(dir string) (string, error) {
	hash, err := gitx.Run(dir, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("store: read HEAD: %w", err)
	}
	return hash, nil
}
