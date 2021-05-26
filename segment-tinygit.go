package main

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	pwl "github.com/justjanne/powerline-go/powerline"
)

type repoStats2 struct {
	ahead      int
	behind     int
	untracked  int
	notStaged  int
	staged     int
	conflicted int
	stashed    int
}

func (r repoStats2) dirty() bool {
	return r.untracked+r.notStaged+r.staged+r.conflicted > 0
}

func (r repoStats2) any() bool {
	return r.ahead+r.behind+r.untracked+r.notStaged+r.staged+r.conflicted+r.stashed > 0
}

func computeRemoteDiff(repo *git.Repository) (int, int) {
	// Get the curent branch name
	local_ref, _ := repo.Head()
	branch := local_ref.Name().Short()
	conf, _ := repo.Config()
	branch_conf := conf.Branches[branch]
	remote_ref_name := plumbing.NewRemoteReferenceName(branch_conf.Remote, branch_conf.Merge.Short())
	// find the hash tag of the remote reference.
	remote_ref, _ := repo.Reference(remote_ref_name, true)

	// Case 1: local and remote are equal.
	ahead, behind := 0, 0
	if local_ref == remote_ref {
		return ahead, behind
	}
	// Case 2: local either behind or ahead of remote
	var remote_commits, local_commits []plumbing.Hash
	local_log, _ := repo.Log(&git.LogOptions{
		From: local_ref.Hash(),
	})
	remote_log, _ := repo.Log(&git.LogOptions{
		From: remote_ref.Hash(),
	})
	local_done, remote_done := false, false
	for {
		if local_done && remote_done {
			break
		}
		local_commit, err := local_log.Next()
		if err == nil {
			local_commits = append(local_commits, local_commit.Hash)
		} else {
			local_done = true
		}
		remote_commit, err := remote_log.Next()
		if err == nil {
			remote_commits = append(remote_commits, remote_commit.Hash)
		} else {
			remote_done = true
		}
		if !local_done && local_commit.Hash == remote_ref.Hash() {
			return 0, len(local_commits) - 1
		} else if !remote_done && remote_commit.Hash == local_ref.Hash() {
			return len(local_commits) - 1, 0
		}
	}
	// Case 3: local and remote mismatch, there exists biforcation. We do not
	// handle this currently. But it is a good idea to compute the fork point.
	return 0, 0
}

func addRepoStatsSegment2(nChanges int, symbol string, foreground uint8, background uint8) []pwl.Segment {
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

func (r repoStats2) GitSegments2(p *powerline) (segments []pwl.Segment) {
	segments = append(segments, addRepoStatsSegment2(r.ahead, p.symbols.RepoAhead, p.theme.GitAheadFg, p.theme.GitAheadBg)...)
	segments = append(segments, addRepoStatsSegment2(r.behind, p.symbols.RepoBehind, p.theme.GitBehindFg, p.theme.GitBehindBg)...)
	segments = append(segments, addRepoStatsSegment2(r.staged, p.symbols.RepoStaged, p.theme.GitStagedFg, p.theme.GitStagedBg)...)
	segments = append(segments, addRepoStatsSegment2(r.notStaged, p.symbols.RepoNotStaged, p.theme.GitNotStagedFg, p.theme.GitNotStagedBg)...)
	segments = append(segments, addRepoStatsSegment2(r.untracked, p.symbols.RepoUntracked, p.theme.GitUntrackedFg, p.theme.GitUntrackedBg)...)
	segments = append(segments, addRepoStatsSegment2(r.conflicted, p.symbols.RepoConflicted, p.theme.GitConflictedFg, p.theme.GitConflictedBg)...)
	segments = append(segments, addRepoStatsSegment2(r.stashed, p.symbols.RepoStashed, p.theme.GitStashedFg, p.theme.GitStashedBg)...)
	return
}

func addRepoStatsSymbol2(nChanges int, symbol string, GitMode string) string {
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

func (r repoStats2) GitSymbols(p *powerline) string {
	var info string
	info += addRepoStatsSymbol2(r.ahead, p.symbols.RepoAhead, p.cfg.GitMode)
	info += addRepoStatsSymbol2(r.behind, p.symbols.RepoBehind, p.cfg.GitMode)
	info += addRepoStatsSymbol2(r.staged, p.symbols.RepoStaged, p.cfg.GitMode)
	info += addRepoStatsSymbol2(r.notStaged, p.symbols.RepoNotStaged, p.cfg.GitMode)
	info += addRepoStatsSymbol2(r.untracked, p.symbols.RepoUntracked, p.cfg.GitMode)
	info += addRepoStatsSymbol2(r.conflicted, p.symbols.RepoConflicted, p.cfg.GitMode)
	info += addRepoStatsSymbol2(r.stashed, p.symbols.RepoStashed, p.cfg.GitMode)
	return info
}

func parseGitStats2(status []string) repoStats2 {
	stats := repoStats2{}
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

func segmentTinygit(p *powerline) []pwl.Segment {
	repo, err := git.PlainOpenWithOptions(p.cwd, &git.PlainOpenOptions{
		DetectDotGit:          true,
		EnableDotGitCommonDir: true,
	})
	if err != nil {
		return []pwl.Segment{}
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return []pwl.Segment{}
	}
	treeStats, err := worktree.Status()
	if err != nil {
		return []pwl.Segment{}
	}

	status := strings.Split(treeStats.String(), "\n")
	stats := parseGitStats2(status)

	ref, err := repo.Head()
	if err != nil {
		return []pwl.Segment{}
	}
	ref_spec := ref.Strings() // 0 as symbolic, 1 as hex
	var branch string
	if len(ref_spec[0]) >= 11 && ref_spec[0][0:11] == "refs/heads/" {
		// the same as `git symbolic-ref --short HEAD`
		branch = strings.TrimPrefix(ref_spec[0], "refs/heads/")
	} else {
		// the same as `git rev-parse --short HEAD`
		branch = ref_spec[1][0:7]
	}

	if len(p.symbols.RepoBranch) > 0 {
		branch = fmt.Sprintf("%s %s", p.symbols.RepoBranch, branch)
	}

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

	want_ahead_behind := true
	for _, stat := range p.cfg.GitDisableStats {
		// "ahead, behind, staged, notStaged, untracked, conflicted, stashed"
		switch stat {
		case "ahead":
			stats.ahead = 0
			want_ahead_behind = false
		case "behind":
			stats.behind = 0
			want_ahead_behind = false
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
		}
	}
	if want_ahead_behind {
		stats.behind, stats.ahead = computeRemoteDiff(repo)
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
		segments = append(segments, stats.GitSegments2(p)...)
	}

	return segments
}
