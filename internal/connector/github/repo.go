package github

import "time"

type Repo struct {
	Name          string    `json:"name"`
	FullName      string    `json:"full_name"`
	Description   string    `json:"description"`
	HTMLURL       string    `json:"html_url"`
	CloneURL      string    `json:"clone_url"`
	DefaultBranch string    `json:"default_branch"`
	Archived      bool      `json:"archived"`
	Fork          bool      `json:"fork"`
	Topics        []string  `json:"topics"`
	StarCount     int       `json:"stargazers_count"`
	PushedAt      time.Time `json:"pushed_at"`
}

type treeResponse struct {
	SHA       string     `json:"sha"`
	Tree      []treeNode `json:"tree"`
	Truncated bool       `json:"truncated"`
}

type treeNode struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

type branchResponse struct {
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

type fileContentResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}
