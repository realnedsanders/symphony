// Package gitx runs git in a repository directory.
package gitx

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Run executes git in dir and returns trimmed stdout.
func Run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = env()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), msg, err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// RunAt executes git with -C omitted, for commands that take the directory
// themselves (worktree add). Dir is the process working directory.
func RunAt(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = env()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), msg, err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func env() []string {
	return append(os.Environ(),
		"GIT_AUTHOR_NAME=Symphony",
		"GIT_AUTHOR_EMAIL=symphony@local",
		"GIT_COMMITTER_NAME=Symphony",
		"GIT_COMMITTER_EMAIL=symphony@local",
	)
}

// EnsureRepo initializes dir as a git repository if it is not one yet.
func EnsureRepo(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	gitDir := dir + "/.git"
	if st, err := os.Stat(gitDir); err == nil && (st.IsDir() || st.Mode().IsRegular()) {
		return configure(dir)
	}
	if _, err := Run(dir, "init", "-b", "main"); err != nil {
		return err
	}
	return configure(dir)
}

func configure(dir string) error {
	if _, err := Run(dir, "config", "user.email", "symphony@local"); err != nil {
		return err
	}
	_, err := Run(dir, "config", "user.name", "Symphony")
	return err
}
