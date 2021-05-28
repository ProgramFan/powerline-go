package main

import (
	"github.com/go-git/go-git/v5"

	pwl "github.com/justjanne/powerline-go/powerline"
)

func segmentGitLite(p *powerline) []pwl.Segment {
	repo, err := git.PlainOpenWithOptions(p.cwd, &git.PlainOpenOptions{
		DetectDotGit:          true,
		EnableDotGitCommonDir: true,
	})
	if err != nil {
		return []pwl.Segment{}
	}
	if len(p.ignoreRepos) > 0 {
		root := repoRoot(repo)
		if p.ignoreRepos[root] {
			return []pwl.Segment{}
		}
	}

	branch := repoBranch(repo)
	return []pwl.Segment{{
		Name:       "git-branch",
		Content:    branch,
		Foreground: p.theme.RepoCleanFg,
		Background: p.theme.RepoCleanBg,
	}}
}
