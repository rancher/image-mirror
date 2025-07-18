package git

import (
	"fmt"
	"os/exec"
	"strings"
)

func IsWorkingTreeClean() (bool, error) {
	cmd := exec.Command("git", "diff", "--quiet")
	err := cmd.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		} else {
			return false, fmt.Errorf("failed to run git diff: %w", err)
		}
	}
	return true, nil
}

func CreateAndCheckoutBranch(baseBranch, branchName string) error {
	baseCheckoutCmd := exec.Command("git", "checkout", baseBranch)
	if err := baseCheckoutCmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", baseBranch, err)
	}

	newCheckoutCmd := exec.Command("git", "checkout", "-b", branchName)
	if err := newCheckoutCmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout new branch: %w", err)
	}

	return nil
}

func Commit(msg string) error {
	cmd := exec.Command("git", "commit", "--all", "--message", msg)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run git commit: %w", err)
	}
	return nil
}

func PushBranch(branchName, remote string) error {
	cmd := exec.Command("git", "push", remote, branchName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run git push: %w", err)
	}
	return nil
}

func GetMergeBase(branch string) (string, error) {
	cmd := exec.Command("git", "merge-base", "HEAD", branch)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get merge base with %s: %w", branch, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func GetFileContentAtCommit(commit, filePath string) ([]byte, error) {
	cmd := exec.Command("git", "show", fmt.Sprintf("%s:%s", commit, filePath))
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get file content for %s at commit %s: %w", filePath, commit, err)
	}
	return out, nil
}
