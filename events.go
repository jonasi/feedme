package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/buger/goterm"
	"github.com/octokit/go-octokit/octokit"
)

const (
	TypeCommitCommentEvent            = "CommitCommentEvent"
	TypeCreateEvent                   = "CreateEvent"
	TypeDeleteEvent                   = "DeleteEvent"
	TypeDeploymentEvent               = "DeploymentEvent"
	TypeDeploymentStatusEvent         = "DeploymentStatusEvent"
	TypeDownloadEvent                 = "DownloadEvent"
	TypeFollowEvent                   = "FollowEvent"
	TypeForkEvent                     = "ForkEvent"
	TypeForkApplyEvent                = "ForkApplyEvent"
	TypeGistEvent                     = "GistEvent"
	TypeGollumEvent                   = "GollumEvent"
	TypeIssueCommentEvent             = "IssueCommentEvent"
	TypeIssuesEvent                   = "IssuesEvent"
	TypeMemberEvent                   = "MemberEvent"
	TypeMembershipEvent               = "MembershipEvent"
	TypePageBuildEvent                = "PageBuildEvent"
	TypePublicEvent                   = "PublicEvent"
	TypePullRequestEvent              = "PullRequestEvent"
	TypePullRequestReviewCommentEvent = "PullRequestReviewCommentEvent"
	TypePushEvent                     = "PushEvent"
	TypeReleaseEvent                  = "ReleaseEvent"
	TypeRepositoryEvent               = "RepositoryEvent"
	TypeStatusEvent                   = "StatusEvent"
	TypeTeamAddEvent                  = "TeamAddEvent"
	TypeWatchEvent                    = "WatchEvent"
)

type event struct {
	Id        string                `json:"id"`
	Actor     *octokit.User         `json:"actor"`
	Type      string                `json:"type"`
	Public    bool                  `json:"public"`
	Repo      *octokit.Repository   `json:"repo"`
	Org       *octokit.Organization `json:"org"`
	CreatedAt time.Time             `json:"created_at"`
}

type Event struct {
	*event
	Payload EventPayload
}

type EventPayload interface {
	Summary(*Event) string
}

func (e *Event) Summary() string {
	var sum string

	if e.Payload != nil {
		sum = e.Payload.Summary(e)
	} else {
		sum = "Unhandled event [" + e.Type + "]"
	}

	width := goterm.Width()
	d := e.CreatedAt.Local().Format("Jan 2 3:04:05 PM")

	lines := strings.Split(sum, "\n")
	lines[0] = fmt.Sprintf("%-30s%s%"+strconv.Itoa(width-30-len([]rune(lines[0])))+"s", e.Repo.Name, lines[0], d)

	return strings.Join(lines, "\n")
}

type jsonEvent struct {
	event
	Payload json.RawMessage `json:"payload"`
}

func (e *Event) UnmarshalJSON(data []byte) error {
	var je jsonEvent

	if err := json.Unmarshal(data, &je); err != nil {
		return err
	}

	e.event = &je.event

	switch e.Type {
	case TypeCommitCommentEvent:
	case TypeCreateEvent:
		e.Payload = &CreateEvent{}
	case TypeDeleteEvent:
		e.Payload = &DeleteEvent{}
	case TypeDeploymentEvent:
	case TypeDeploymentStatusEvent:
	case TypeDownloadEvent:
	case TypeFollowEvent:
	case TypeForkEvent:
	case TypeForkApplyEvent:
	case TypeGistEvent:
	case TypeGollumEvent:
		e.Payload = &GollumEvent{}
	case TypeIssueCommentEvent:
		e.Payload = &IssueCommentEvent{}
	case TypeIssuesEvent:
		e.Payload = &IssuesEvent{}
	case TypeMemberEvent:
	case TypeMembershipEvent:
	case TypePageBuildEvent:
	case TypePublicEvent:
	case TypePullRequestEvent:
		e.Payload = &PullRequestEvent{}
	case TypePullRequestReviewCommentEvent:
		e.Payload = &PullRequestReviewCommentEvent{}
	case TypePushEvent:
		e.Payload = &PushEvent{}
	case TypeReleaseEvent:
	case TypeRepositoryEvent:
	case TypeStatusEvent:
	case TypeTeamAddEvent:
	case TypeWatchEvent:
	}

	if e.Payload != nil {
		return json.Unmarshal(je.Payload, &e.Payload)
	}

	return nil
}

type CreateEvent struct {
	RefType      string `json:"ref_type"`
	Ref          string `json:"ref"`
	MasterBranch string `json:"master_branch"`
	Description  string `json:"description"`
}

func (p *CreateEvent) Summary(ev *Event) string {
	return fmt.Sprintf("@%s created a new %s: %s", ev.Actor.Login, p.RefType, p.Ref)
}

type IssueCommentEvent struct {
	Action  string        `json:"action"`
	Issue   octokit.Issue `json:"issue"`
	Comment Comment       `json:"comment"`
}

func (p *IssueCommentEvent) Summary(ev *Event) string {
	return fmt.Sprintf("@%s commented on issue #%d\n\n%s", ev.Actor.Login, p.Issue.Number, p.Comment.Body)
}

type IssuesEvent struct {
	Action   string        `json:"action"`
	Issue    octokit.Issue `json:"issue"`
	Assignee *octokit.User `json:"assignee"`
	Label    *string       `json:"label"`
}

func (p *IssuesEvent) Summary(ev *Event) string {
	return fmt.Sprintf("@%s %s #%d", ev.Actor.Login, p.Action, p.Issue.Number)
}

type PullRequestEvent struct {
	Action      string              `json:"action'`
	Number      int                 `json:"number"`
	PullRequest octokit.PullRequest `json:"pull_request"`
}

func (p *PullRequestEvent) Summary(ev *Event) string {
	return fmt.Sprintf("@%s %s a pull request #%d", ev.Actor.Login, p.Action, p.PullRequest.Number)
}

type PushEvent struct {
	Head         string   `json:"head"`
	Ref          string   `json:"ref"`
	Size         int      `json:"size"`
	DistinctSize int      `json:"distinct_size"`
	Commits      []Commit `json:"commits"`
}

func (p *PushEvent) Summary(ev *Event) string {
	c := "commits"
	if p.Size == 1 {
		c = "commit"
	}

	ref := strings.Replace(p.Ref, "refs/heads/", "", -1)
	str := fmt.Sprintf("@%s pushed %d %s to %s\n", ev.Actor.Login, p.DistinctSize, c, ref)
	i := 0

	for _, c := range p.Commits {
		if c.Distinct {
			str += fmt.Sprintf("\n%s %s", c.Sha[:8], c.Message)
			i++
		}
	}

	return str
}

type PullRequestReviewCommentEvent struct {
	Action      string              `json:"action"`
	PullRequest octokit.PullRequest `json:"pull_request"`
	Comment     Comment             `json:"comment"`
}

func (p *PullRequestReviewCommentEvent) Summary(ev *Event) string {
	return fmt.Sprintf("@%s commented on pull request #%d\n\n%s", ev.Actor.Login, p.PullRequest.Number, p.Comment.Body)
}

type GollumEvent struct {
	Pages []Page `json:"pages"`
}

func (p *GollumEvent) Summary(ev *Event) string {
	return fmt.Sprintf("@%s modified %d wiki pages", ev.Actor.Login, len(p.Pages))
}

type DeleteEvent struct {
	RefType string `json:"ref_type"`
	Ref     string `json:"ref"`
}

func (p *DeleteEvent) Summary(ev *Event) string {
	return fmt.Sprintf("@%s deleted %s %s", ev.Actor.Login, p.RefType, p.Ref)
}

type Commit struct {
	Sha      string       `json:"sha"`
	Message  string       `json:"message"`
	Author   CommitAuthor `json:"author"`
	Url      string       `json:"url"`
	Distinct bool         `json:"distinct"`
}

type CommitAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Comment struct {
	Id        int          `json:"id"`
	Url       string       `json:"url"`
	HtmlUrl   string       `json:"html_url"`
	Body      string       `json:"body"`
	User      octokit.User `json:"user"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

type Page struct {
	Name    string `json:"page_name"`
	Title   string `json:"title"`
	Action  string `json:"action"`
	Sha     string `json:"sha"`
	HtmlUrl string `json:"html_url"`
}
