package sdk

import (
	"bytes"
	"time"
)

type VCSServer interface {
	AuthorizeRedirect() (string, string, error)
	AuthorizeToken(string, string) (string, string, error)
	GetAuthorizedClient(string, string) (VCSAuthorizedClient, error)
}

type VCSAuthorizedClient interface {
	//Repos
	Repos() ([]VCSRepo, error)
	RepoByFullname(fullname string) (VCSRepo, error)

	//Branches
	Branches(string) ([]VCSBranch, error)
	Branch(string, string) (*VCSBranch, error)

	//Commits
	Commits(repo, branch, since, until string) ([]VCSCommit, error)
	Commit(repo, hash string) (VCSCommit, error)

	// PullRequests
	PullRequests(string) ([]VCSPullRequest, error)

	//Hooks
	CreateHook(repo, url string) error
	DeleteHook(repo, url string) error

	//Events
	GetEvents(repo string, dateRef time.Time) ([]interface{}, time.Duration, error)
	PushEvents(string, []interface{}) ([]VCSPushEvent, error)
	CreateEvents(string, []interface{}) ([]VCSCreateEvent, error)
	DeleteEvents(string, []interface{}) ([]VCSDeleteEvent, error)
	PullRequestEvents(string, []interface{}) ([]VCSPullRequestEvent, error)

	// Set build status on repository
	SetStatus(event Event) error

	// Release
	Release(repo, tagName, releaseTitle, releaseDescription string) (*VCSRelease, error)
	UploadReleaseFile(repo string, release *VCSRelease, runArtifact WorkflowNodeRunArtifact, file *bytes.Buffer) error
}