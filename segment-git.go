package main

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	pwl "github.com/justjanne/powerline-go/powerline"
)

// Detailed repository and working tree status
type repoStats struct {
	ahead      int
	behind     int
	untracked  int
	notStaged  int
	staged     int
	conflicted int
	stashed    int
}

// Check if the working tree is dirty
func (r repoStats) dirty() bool {
	return r.untracked+r.notStaged+r.staged+r.conflicted > 0
}

// Check if there are any useful details to display
func (r repoStats) any() bool {
	return r.ahead+r.behind+r.untracked+r.notStaged+r.staged+r.conflicted+r.stashed > 0
}

// Build a segment representing one repo stats
func addRepoStatsSegment(nChanges int, symbol string, foreground uint8, background uint8) []pwl.Segment {
	if nChanges > 0 {
		return []pwl.Segment{{
			Name:       "git-status",
			Content:    fmt.Sprintf("%d%s", nChanges, symbol),
			Foreground: foreground,
			Background: background,
		}}
	}
	return []pwl.Segment{}
}

// Build segments representing the stats
func (r repoStats) GitSegments(p *powerline) (segments []pwl.Segment) {
	segments = append(segments, addRepoStatsSegment(r.ahead, p.symbols.RepoAhead, p.theme.GitAheadFg, p.theme.GitAheadBg)...)
	segments = append(segments, addRepoStatsSegment(r.behind, p.symbols.RepoBehind, p.theme.GitBehindFg, p.theme.GitBehindBg)...)
	segments = append(segments, addRepoStatsSegment(r.staged, p.symbols.RepoStaged, p.theme.GitStagedFg, p.theme.GitStagedBg)...)
	segments = append(segments, addRepoStatsSegment(r.notStaged, p.symbols.RepoNotStaged, p.theme.GitNotStagedFg, p.theme.GitNotStagedBg)...)
	segments = append(segments, addRepoStatsSegment(r.untracked, p.symbols.RepoUntracked, p.theme.GitUntrackedFg, p.theme.GitUntrackedBg)...)
	if r.stashed > 0 {
		seg := addRepoStatsSegment(r.stashed, p.symbols.RepoStashed, p.theme.GitStashedFg, p.theme.GitStashedBg)
		seg[0].Content = fmt.Sprintf("%s", p.symbols.RepoStashed)
		segments = append(segments, seg...)
	}
	return
}

func addRepoStatsSymbol(nChanges int, symbol string, GitMode string) string {
	if nChanges > 0 {
		if GitMode == "simple" {
			return symbol
		} else if GitMode == "compact" {
			return fmt.Sprintf(" %d%s", nChanges, symbol)
		} else {
			return symbol
		}
	}
	return ""
}

func (r repoStats) GitSymbols(p *powerline) string {
	var info string
	info += addRepoStatsSymbol(r.ahead, p.symbols.RepoAhead, p.cfg.GitMode)
	info += addRepoStatsSymbol(r.behind, p.symbols.RepoBehind, p.cfg.GitMode)
	info += addRepoStatsSymbol(r.staged, p.symbols.RepoStaged, p.cfg.GitMode)
	info += addRepoStatsSymbol(r.notStaged, p.symbols.RepoNotStaged, p.cfg.GitMode)
	info += addRepoStatsSymbol(r.untracked, p.symbols.RepoUntracked, p.cfg.GitMode)
	info += addRepoStatsSymbol(r.conflicted, p.symbols.RepoConflicted, p.cfg.GitMode)
	info += addRepoStatsSymbol(r.stashed, p.symbols.RepoStashed, p.cfg.GitMode)
	return info
}

func parseGitStats(status []string) repoStats {
	stats := repoStats{}
	if len(status) > 1 {
		for _, line := range status {
			if len(line) > 2 {
				code := line[:2]
				switch code {
				case "??":
					stats.untracked++
				case "DD", "AU", "UD", "UA", "DU", "AA", "UU":
					stats.conflicted++
				default:
					if code[0] != ' ' {
						stats.staged++
					}

					if code[1] != ' ' {
						stats.notStaged++
					}
				}
			}
		}
	}
	return stats
}

// Get the root of a repository.
func repoRoot(repo *git.Repository) string {
	tree, _ := repo.Worktree()
	return strings.TrimSpace(tree.Filesystem.Root())
}

// Check if a repo has stash commits. Due to the inability of go-git to parse
// reflog, we can only check if the repo has stash, and can not determine how
// many stash are there.
func repoHasStash(repo *git.Repository) bool {
	_, err := repo.Reference("refs/stash", true)
	return err == nil
}

// Determine the difference of current branch and its upstream branch. Returns
// (ahead, behind) denoting how many commits ahead of the remote and how many
// commits behind the remote.
func repoAheadBehind(repo *git.Repository) (int, int) {
	// Generate the local and remote branch reference
	localRef, _ := repo.Head()
	if !localRef.Name().IsBranch() {
		return 0, 0
	}
	conf, _ := repo.Config()
	branch := localRef.Name().Short()
	branchConf, ok := conf.Branches[branch]
	if !ok {
		return 0, 0
	}
	remoteBranchName := plumbing.NewRemoteReferenceName(
		branchConf.Remote, branchConf.Merge.Short())
	remoteRef, err := repo.Reference(remoteBranchName, true)
	if err != nil || !remoteRef.Name().IsRemote() {
		return 0, 0
	}

	// Determine the difference. We assume the local and remote branch orginate
	// from the same initial commit (always true if the local branch tracks the
	// remote branch). The algorithm first checks if there is no diffrence since
	// it is the most common case. It then checks if the local is either ahead
	// of or behind the remote by comparing the history with the branch head. At
	// the same time, it populates the history list. If this step fails, the
	// branches go into different ways. So the final step is to find the latest
	// common roots of the two branches by traversing the history. The algorithm
	// will always return meaningful results.

	// Case 1: local and remote are equal.
	ahead, behind := 0, 0
	if localRef == remoteRef {
		return ahead, behind
	}
	// Case 2: local either behind or ahead of remote
	var remoteCommits, localCommits []plumbing.Hash
	localLog, _ := repo.Log(&git.LogOptions{
		From: localRef.Hash(),
	})
	remoteLog, _ := repo.Log(&git.LogOptions{
		From: remoteRef.Hash(),
	})
	localEOI, remoteEOI := false, false
	var localCommit, remoteCommit *object.Commit
	for {
		if localEOI && remoteEOI {
			break
		}
		if !localEOI {
			localCommit, err = localLog.Next()
			if err == nil {
				localCommits = append(localCommits, localCommit.Hash)
			} else {
				localEOI = true
			}
		}
		if !remoteEOI {
			remoteCommit, err = remoteLog.Next()
			if err == nil {
				remoteCommits = append(remoteCommits, remoteCommit.Hash)
			} else {
				remoteEOI = true
			}
		}
		if !localEOI && localCommit.Hash == remoteRef.Hash() {
			return len(localCommits) - 1, 0
		} else if !remoteEOI && remoteCommit.Hash == localRef.Hash() {
			return 0, len(localCommits) - 1
		}
	}
	// Case 3: local and remote mismatch, there exists biforcation. We find the
	// biforcation from the beginning of the two lists.
	localCommitsCount := len(localCommits)
	remoteCommitsCount := len(remoteCommits)
	bound := localCommitsCount
	if remoteCommitsCount < bound {
		bound = remoteCommitsCount
	}
	i := 0
	for ; i < bound; i++ {
		localCommit := localCommits[localCommitsCount-1-i]
		remoteCommit := remoteCommits[remoteCommitsCount-1-i]
		if localCommit != remoteCommit {
			break
		}
	}
	return localCommitsCount - i, remoteCommitsCount - i
}

func repoBranch(repo *git.Repository) string {
	ref, err := repo.Head()
	if err != nil {
		return ""
	}
	if ref.Name().IsBranch() {
		return ref.Name().Short()
	} else {
		return ref.Hash().String()[:7]
	}
}

func repoStatus(repo *git.Repository) repoStats {
	var stats repoStats
	tree, err := repo.Worktree()
	if err != nil {
		return stats
	}
	treeStats, err := tree.Status()
	if err != nil {
		return stats
	}
	status := strings.Split(treeStats.String(), "\n")
	return parseGitStats(status)
}

func segmentGit(p *powerline) []pwl.Segment {
	repo, err := git.PlainOpenWithOptions(p.cwd, &git.PlainOpenOptions{
		DetectDotGit:          true,
		EnableDotGitCommonDir: true,
	})
	if err != nil {
		return []pwl.Segment{}
	}
	root := repoRoot(repo)
	if len(p.ignoreRepos) > 0 && p.ignoreRepos[root] {
		return []pwl.Segment{}
	}

	branch := repoBranch(repo)
	if len(p.symbols.RepoBranch) > 0 {
		branch = fmt.Sprintf("%s %s", p.symbols.RepoBranch, branch)
	}

	stats := repoStatus(repo)
	var foreground, background uint8
	if stats.dirty() {
		foreground = p.theme.RepoDirtyFg
		background = p.theme.RepoDirtyBg
	} else {
		foreground = p.theme.RepoCleanFg
		background = p.theme.RepoCleanBg
	}

	segments := []pwl.Segment{{
		Name:       "git-branch",
		Content:    branch,
		Foreground: foreground,
		Background: background,
	}}

	askedAheadBehind := true
	askedStash := true
	for _, stat := range p.cfg.GitDisableStats {
		// "ahead, behind, staged, notStaged, untracked, conflicted, stashed"
		switch stat {
		case "ahead":
			stats.ahead = 0
			askedAheadBehind = false
		case "behind":
			stats.behind = 0
			askedAheadBehind = false
		case "staged":
			stats.staged = 0
		case "notStaged":
			stats.notStaged = 0
		case "untracked":
			stats.untracked = 0
		case "conflicted":
			stats.conflicted = 0
		case "stashed":
			stats.stashed = 0
			askedStash = false
		}
	}
	if askedAheadBehind {
		stats.ahead, stats.behind = repoAheadBehind(repo)
	}
	if askedStash && repoHasStash(repo) {
		stats.stashed = 1
	}

	if p.cfg.GitMode == "simple" {
		if stats.any() {
			segments[0].Content += " " + stats.GitSymbols(p)
		}
	} else if p.cfg.GitMode == "compact" {
		if stats.any() {
			segments[0].Content += stats.GitSymbols(p)
		}
	} else { // fancy
		segments = append(segments, stats.GitSegments(p)...)
	}

	return segments
}
