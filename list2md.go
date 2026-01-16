package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Repo describes a Github repository with additional field, last commit date
type Repo struct {
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	DefaultBranch  string    `json:"default_branch"`
	Stars          int       `json:"stargazers_count"`
	Created        time.Time `json:"created_at"`
	Updated        time.Time `json:"updated_at"`
	URL            string    `json:"html_url"`
	LastCommitDate time.Time `json:"-"`
}

// HeadCommit describes a head commit of default branch
type HeadCommit struct {
	Sha    string `json:"sha"`
	Commit struct {
		Committer struct {
			Name  string    `json:"name"`
			Email string    `json:"email"`
			Date  time.Time `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}

const (
	head = `# Top Go Web Frameworks
A list of popular github projects related to Go web framework (ranked by stars automatically)
Please update **list.txt** (via Pull Request)

| Project Name | Stars | Description | Last Commit |
| ------------ | ----- | ----------- | ----------- |
`
	tail = "\n*Last Automatic Update: %v*"

	warning = "⚠️ No longer maintained ⚠️  "
)

var (
	deprecatedRepos = []string{"https://github.com/go-martini/martini", "https://github.com/pilu/traffic"}
)

func main() {
	accessToken := getAccessToken()

	byteContents, err := os.ReadFile("list.txt")
	if err != nil {
		log.Fatalf("Failed to read list.txt: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(byteContents)), "\n")
	var repos []Repo

	for _, url := range lines {
		url = strings.TrimSpace(url)
		if url == "" || !strings.HasPrefix(url, "https://github.com/") {
			continue
		}

		repoPath := strings.TrimFunc(url[19:], trimSpaceAndSlash)
		if repoPath == "" {
			continue
		}

		fmt.Printf("Processing: %s\n", url)

		repo, err := fetchRepo(repoPath, accessToken)
		if err != nil {
			log.Printf("Failed to fetch repo %s: %v", url, err)
			continue
		}

		commit, err := fetchLastCommit(repoPath, repo.DefaultBranch, accessToken)
		if err != nil {
			log.Printf("Failed to fetch commit for %s: %v", url, err)
			continue
		}

		repo.LastCommitDate = commit.Commit.Committer.Date
		repos = append(repos, repo)

		fmt.Printf("✓ %s: %d stars\n", repo.Name, repo.Stars)
		time.Sleep(3 * time.Second)
	}

	sort.Slice(repos, func(i, j int) bool {
		return repos[i].Stars > repos[j].Stars
	})

	if err := saveRanking(repos); err != nil {
		log.Fatalf("Failed to save ranking: %v", err)
	}

	fmt.Printf("\n✓ Successfully updated %d repositories\n", len(repos))
}

func trimSpaceAndSlash(r rune) bool {
	return unicode.IsSpace(r) || r == '/'
}

func getAccessToken() string {
	tokenBytes, err := os.ReadFile("access_token.txt")
	if err != nil {
		log.Fatalf("Failed to read access_token.txt: %v", err)
	}
	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		log.Fatal("Access token is empty")
	}
	return token
}

func fetchRepo(repoPath, accessToken string) (Repo, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s", repoPath)
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return Repo{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Repo{}, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Repo{}, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var repo Repo
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		return Repo{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return repo, nil
}

func fetchLastCommit(repoPath, branch, accessToken string) (HeadCommit, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/commits/%s", repoPath, branch)
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return HeadCommit{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return HeadCommit{}, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return HeadCommit{}, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var commit HeadCommit
	if err := json.NewDecoder(resp.Body).Decode(&commit); err != nil {
		return HeadCommit{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return commit, nil
}

func formatTimeAgo(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	days := int(diff.Hours() / 24)
	weeks := days / 7
	months := days / 30
	years := days / 365

	switch {
	case years > 0:
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	case months > 0:
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	case weeks > 0:
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	case days > 0:
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		hours := int(diff.Hours())
		if hours > 0 {
			if hours == 1 {
				return "1 hour ago"
			}
			return fmt.Sprintf("%d hours ago", hours)
		}
		minutes := int(diff.Minutes())
		if minutes > 0 {
			if minutes == 1 {
				return "1 minute ago"
			}
			return fmt.Sprintf("%d minutes ago", minutes)
		}
		return "just now"
	}
}

func saveRanking(repos []Repo) error {
	file, err := os.OpenFile("README.md", os.O_RDWR|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("failed to open README.md: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(head); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	for _, repo := range repos {
		description := repo.Description
		if isDeprecated(repo.URL) {
			description = warning + description
		}
		timeAgo := formatTimeAgo(repo.LastCommitDate)
		line := fmt.Sprintf("| [%s](%s) | %d | %s | %s |\n",
			repo.Name, repo.URL, repo.Stars, description, timeAgo)
		if _, err := file.WriteString(line); err != nil {
			return fmt.Errorf("failed to write repo line: %w", err)
		}
	}

	tailLine := fmt.Sprintf(tail, time.Now().Format(time.RFC3339))
	if _, err := file.WriteString(tailLine); err != nil {
		return fmt.Errorf("failed to write tail: %w", err)
	}

	return nil
}

func isDeprecated(repoURL string) bool {
	for _, deprecatedRepo := range deprecatedRepos {
		if repoURL == deprecatedRepo {
			return true
		}
	}
	return false
}
