package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/antonmedv/gitmal/pkg/templates"
)

func generatePortalIndex(repos []templates.RepoSummary, outputRoot string, owner string, dark bool) error {
	outputDir, err := filepath.Abs(outputRoot)
	if err != nil {
		return err
	}

	sorted := make([]templates.RepoSummary, len(repos))
	copy(sorted, repos)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].DisplayName < sorted[j].DisplayName
	})

	sort.SliceStable(sorted, func(i, j int) bool {
		dateI := sorted[i].LastCommit.Date
		dateJ := sorted[j].LastCommit.Date
		if dateI.IsZero() && dateJ.IsZero() {
			return sorted[i].DisplayName < sorted[j].DisplayName
		}
		if dateI.IsZero() {
			return false
		}
		if dateJ.IsZero() {
			return true
		}
		return dateI.After(dateJ)
	})

	heading := "Ansari Repos"
	title := "Ansari Repos"
	headerName := "Ansari Repos"
	if owner != "" {
		heading = owner
		title = owner + " / Repositories"
		headerName = owner
	}

	f, err := os.Create(filepath.Join(outputDir, "index.html"))
	if err != nil {
		return err
	}

	err = templates.ReposTemplate.ExecuteTemplate(f, "layout.gohtml", templates.ReposParams{
		LayoutParams: templates.LayoutParams{
			Title:    title,
			Name:     headerName,
			Dark:     dark,
			RootHref: "./",
		},
		Heading: heading,
		Repos:   sorted,
		Total:   len(sorted),
	})
	if err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	return nil
}

func repoDisplayName(owner, name string) string {
	if owner != "" {
		return owner + "/" + name
	}
	return name
}

func getRepoDescriptionFromGitHubAPI(owner, repo string) string {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)

	resp, err := http.Get(apiURL)
	if err != nil {
		fmt.Println("Error making HTTP request:", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("GitHub API returned non-OK status:", resp.Status)
		return ""
	}

	var result struct {
		Description string `json:"description"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		fmt.Println("Error decoding JSON response:", err)
		return ""
	}

	return result.Description

}

func buildRepoSummary(params Params, defaultBranch string, branches int, tags int, details templates.CommitDetails) templates.RepoSummary {

	description := getRepoDescriptionFromGitHubAPI("The-Robin-Hood", params.Name)
	if description == "" {
		descriptionFile := filepath.Join(params.RepoDir, ".git", "description")
		descriptionCmd := exec.Command("git", "-C", params.RepoDir, "rev-parse", "--git-dir")
		output, err := descriptionCmd.Output()
		if err != nil {
			fmt.Println("Error executing git rev-parse command:", err)
		} else {
			gitDir := strings.TrimSpace(string(output))
			descriptionFile = filepath.Join(params.RepoDir, gitDir, "description")
		}
		if _, err := os.Stat(descriptionFile); err == nil {
			content, err := os.ReadFile(descriptionFile)
			if err == nil {
				desc := strings.TrimSpace(string(content))
				if !strings.HasPrefix(desc, "Unnamed repository") && desc != "" {
					description = desc
				}
			}
		}
	}

	summary := templates.RepoSummary{
		Name:          params.Name,
		Description:   description,
		Owner:         params.Owner,
		DisplayName:   repoDisplayName(params.Owner, params.Name),
		Href:          filepath.ToSlash(filepath.Join(params.Name, "index.html")),
		DefaultBranch: defaultBranch,
		BranchCount:   branches,
		TagCount:      tags,
	}
	if details.TotalCommits > 0 {
		summary.TotalCommits = details.TotalCommits
		summary.LastCommit = details.LastCommit
		summary.LastCommitDate = details.LastCommitDate
		summary.LastCommitSubject = strings.TrimSpace(details.LastCommit.Subject)
	}
	return summary
}
