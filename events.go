package main

import (
	"encoding/json"
	"fmt"
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

	const colSize = 30

	width := goterm.Width()
	lines := strings.Split(sum, "\n")
	lines = indent(wrap(lines, width-colSize), colSize)
	d := e.CreatedAt.Local().Format("Jan 2 3:04:05 PM")

	if len(lines) == 1 {
		lines = append(lines, strings.Repeat(" ", width))
	}

	lines[0] = bold(e.Repo.Name) + lines[0][len([]rune(e.Repo.Name)):]
	lines[1] = d + lines[1][len([]rune(d)):]

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
		e.Payload = &CommitCommentEvent{}
	case TypeCreateEvent:
		e.Payload = &CreateEvent{}
	case TypeDeleteEvent:
		e.Payload = &DeleteEvent{}
	case TypeDeploymentEvent:
	case TypeDeploymentStatusEvent:
	case TypeDownloadEvent:
	case TypeFollowEvent:
	case TypeForkEvent:
		e.Payload = &ForkEvent{}
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
		e.Payload = &WatchEvent{}
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
	return fmt.Sprintf(
		"%s created a new %s: %s",
		username(ev.Actor),
		p.RefType,
		p.Ref,
	)
}

type IssueCommentEvent struct {
	Action  string        `json:"action"`
	Issue   octokit.Issue `json:"issue"`
	Comment Comment       `json:"comment"`
}

func (p *IssueCommentEvent) Summary(ev *Event) string {
	return fmt.Sprintf(
		"%s commented on %s\n\n%s\n\n%s",
		username(ev.Actor),
		issue(&p.Issue),
		ellipsis(p.Comment.Body, 5),
		underline(p.Comment.HtmlURL),
	)
}

type IssuesEvent struct {
	Action   string        `json:"action"`
	Issue    octokit.Issue `json:"issue"`
	Assignee *octokit.User `json:"assignee"`
	Label    *string       `json:"label"`
}

func (p *IssuesEvent) Summary(ev *Event) string {
	return fmt.Sprintf(
		"%s %s %s\n\n%s\n\n%s",
		username(ev.Actor),
		p.Action,
		issue(&p.Issue),
		ellipsis(p.Issue.Body, 5),
		underline(p.Issue.HTMLURL),
	)
}

type PullRequestEvent struct {
	Action      string              `json:"action'`
	Number      int                 `json:"number"`
	PullRequest octokit.PullRequest `json:"pull_request"`
}

func (p *PullRequestEvent) Summary(ev *Event) string {
	return fmt.Sprintf(
		"%s %s a pull request %s\n\n%s\n\n%s",
		username(ev.Actor),
		p.Action,
		pr(&p.PullRequest),
		ellipsis(p.PullRequest.Body, 5),
		underline(p.PullRequest.HTMLURL),
	)
}

type PushEvent struct {
	Head         string   `json:"head"`
	Ref          string   `json:"ref"`
	Size         int      `json:"size"`
	Before       string   `json:"before"`
	DistinctSize int      `json:"distinct_size"`
	Commits      []Commit `json:"commits"`
}

func (p *PushEvent) Summary(ev *Event) string {
	c := "commits"
	if p.Size == 1 {
		c = "commit"
	}

	ref := strings.Replace(p.Ref, "refs/heads/", "", -1)
	str := fmt.Sprintf(
		"%s pushed %d %s to %s\n",
		username(ev.Actor),
		p.Size,
		c,
		ref,
	)

	for _, c := range p.Commits {
		str += fmt.Sprintf("\n%s %s", c.Sha[:8], c.Message)
	}

	str += "\n\n"

	if p.Size == 1 {
		str += underline(fmt.Sprintf("https://github.com/%s/commit/%s", ev.Repo.Name, p.Commits[0].Sha))
	} else {
		str += underline(fmt.Sprintf("https://github.com/%s/compare/%s...%s", ev.Repo.Name, p.Before, p.Head))
	}

	return str
}

type PullRequestReviewCommentEvent struct {
	Action      string              `json:"action"`
	PullRequest octokit.PullRequest `json:"pull_request"`
	Comment     Comment             `json:"comment"`
}

func (p *PullRequestReviewCommentEvent) Summary(ev *Event) string {
	return fmt.Sprintf(
		"%s commented on pull request %s\n\n%s\n\n%s",
		username(ev.Actor),
		pr(&p.PullRequest),
		ellipsis(p.Comment.Body, 5),
		underline(p.Comment.HtmlURL),
	)
}

type GollumEvent struct {
	Pages []Page `json:"pages"`
}

func (p *GollumEvent) Summary(ev *Event) string {
	return fmt.Sprintf(
		"%s modified %d wiki pages",
		username(ev.Actor),
		len(p.Pages),
	)
}

type DeleteEvent struct {
	RefType string `json:"ref_type"`
	Ref     string `json:"ref"`
}

func (p *DeleteEvent) Summary(ev *Event) string {
	return fmt.Sprintf(
		"%s deleted %s %s",
		username(ev.Actor),
		p.RefType,
		p.Ref,
	)
}

type WatchEvent struct {
	Action string `json:"action"`
}

func (p *WatchEvent) Summary(ev *Event) string {
	return fmt.Sprintf(
		"%s is watching this repo",
		username(ev.Actor),
	)
}

type ForkEvent struct {
	Forkee octokit.Repository `json:"forkee"`
}

func (p *ForkEvent) Summary(ev *Event) string {
	return fmt.Sprintf(
		"%s forked the repo\n%s",
		username(ev.Actor),
		underline(p.Forkee.HTMLURL),
	)
}

type CommitCommentEvent struct {
	Comment CommitComment `json:"comment"`
}

func (p *CommitCommentEvent) Summary(ev *Event) string {
	return fmt.Sprintf(
		"%s commented on commit %s\n\n%s\n\n%s",
		username(ev.Actor),
		p.Comment.CommitId,
		ellipsis(p.Comment.Body, 5),
		underline(p.Comment.HtmlURL),
	)
}

type CommitComment struct {
	Comment
	CommitId string `json:"commit_id"`
}

type Commit struct {
	Sha      string       `json:"sha"`
	Message  string       `json:"message"`
	Author   CommitAuthor `json:"author"`
	URL      string       `json:"url"`
	Distinct bool         `json:"distinct"`
}

type CommitAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Comment struct {
	Id        int          `json:"id"`
	URL       string       `json:"url"`
	HtmlURL   string       `json:"html_url"`
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
	HtmlURL string `json:"html_url"`
}

func ellipsis(str string, lines int) string {
	l := strings.SplitN(str, "\n", lines+1)

	if len(l) == lines+1 {
		l[lines] = "..."
	}

	return strings.Join(l, "\n")
}

func wrap(lines []string, width int) []string {
	newlines := []string{}

	for _, l := range lines {
		c := len([]rune(l))

		for c > width {
			newlines = append(newlines, l[:width])
			l = l[width:]
			c = len([]rune(l))
		}

		newlines = append(newlines, l)
	}

	return newlines
}

func indent(lines []string, num int) []string {
	for i, l := range lines {
		lines[i] = strings.Repeat(" ", num) + l
	}

	return lines
}

func username(u *octokit.User) string {
	return bold("@" + u.Login)
}

func issue(is *octokit.Issue) string {
	return bold(fmt.Sprintf("#%d [%s]", is.Number, is.Title))
}

func pr(pr *octokit.PullRequest) string {
	return bold(fmt.Sprintf("#%d [%s]", pr.Number, pr.Title))
}

func bold(str string) string {
	return fmt.Sprintf("\033[1m%s\033[0m", str)
}

func underline(str string) string {
	return fmt.Sprintf("\033[4m%s\033[0m", str)
}
