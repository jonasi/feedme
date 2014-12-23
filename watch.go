package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/bgentry/speakeasy"
	"github.com/buger/goterm"
	"github.com/octokit/go-octokit/octokit"
)

var (
	configFile = path.Join(os.Getenv("HOME"), ".config", "github-watch")
	debug      = flag.Bool("debug", false, "")
	orgs       = stringsl{}
	repos      = stringsl{}
)

type auth struct {
	Login string `json:"login"`
	Token string `json:"token"`
}

func main() {
	flag.Var(&orgs, "org", "")
	flag.Parse()

	a := loadFileAuth()

	if a == nil {
		a = promptAuth()
	}

	if a == nil {
		fmt.Println("Could not get an authorization token from Github")
		os.Exit(1)
	}

	saveFileAuth(a)

	u, err := loadUser(a)

	if err != nil {
		fmt.Printf("User load error: %s\n", err)
		os.Exit(1)
	}

	ch := make(chan *pollMsg)

	for _, org := range orgs {
		poll(u, org, a, ch)
	}

	for msg := range ch {
		if msg.err != nil {
			fmt.Printf("Events load error: %s\n", msg.err)
			continue
		}

		width := goterm.Width()
		for i := len(msg.events) - 1; i >= 0; i-- {
			ev := msg.events[i]
			fmt.Println(strings.Repeat("-", width))
			fmt.Println(ev.Summary())
		}
	}
}

func debugf(str string, params ...interface{}) {
	if *debug {
		if str[len(str)-1] != '\n' {
			str += "\n"
		}

		str = "[DEBUG] " + str

		log.Printf(str, params...)
	}
}

func warnf(str string, params ...interface{}) {
	if *debug {
		if str[len(str)-1] != '\n' {
			str += "\n"
		}

		str = "[WARNING] " + str

		log.Printf(str, params...)
	}
}

func loadUser(a *auth) (*octokit.User, error) {
	url, err := octokit.CurrentUserURL.Expand(nil)

	if err != nil {
		return nil, err
	}

	auth := octokit.TokenAuth{a.Token}
	cl := octokit.NewClient(auth)

	u, res := cl.Users(url).One()

	if res.HasError() {
		return nil, res.Err
	}

	return u, nil
}

type pollMsg struct {
	events []Event
	err    error
}

func poll(u *octokit.User, org string, a *auth, send chan *pollMsg) {
	var (
		etag   string
		lastId string
		events []Event
		err    error
		poll   = 30
	)

	go func() {
		events, etag, poll, err = pollEvents(u.Login, org, a, etag, lastId)

		if len(events) > 0 {
			lastId = events[0].Id
		}

		send <- &pollMsg{events, err}

		time.Sleep(time.Duration(poll) * time.Second)
	}()
}

func pollEvents(user string, org string, a *auth, etag, lastId string) ([]Event, string, int, error) {
	var (
		u      string
		events = []Event{}
	)

	if org == "" {
		u = fmt.Sprintf("/users/%s/received_events", user)
	} else {
		u = fmt.Sprintf("/users/%s/events/orgs/%s", user, org)
	}

	var (
		ev   []Event
		poll int
		err  error
		next string
	)

	for {
		ev, etag, poll, next, err = loadEvents(u, a, etag)

		if err != nil {
			return nil, "", -1, err
		}

		idx := -1
		for i, e := range ev {
			if e.Id == lastId {
				idx = i
				break
			}
		}

		if idx >= 0 {
			ev = ev[0:idx]
			next = ""
		}

		events = append(events, ev...)

		if next == "" {
			break
		}

		u = next
	}

	return events, etag, poll, nil
}

func loadEvents(u string, a *auth, etag string) ([]Event, string, int, string, error) {
	var (
		auth = octokit.TokenAuth{a.Token}
		cl   = octokit.NewClient(auth)
	)

	debugf("Polling %s with etag: %s", u, etag)

	req, err := cl.NewRequest(u)

	if err != nil {
		return nil, "", -1, "", err
	}

	if etag != "" {
		req.Header.Add("If-None-Match", etag)
	}

	events := []Event{}

	res, err := req.Get(&events)

	var (
		poll int
		code int
	)

	if res != nil {
		etag = res.Header.Get("Etag")
		poll, _ = strconv.Atoi(res.Header.Get("X-Poll-Interval"))
		code = res.StatusCode
	}

	debugf("Poll Result: code=%d etag=%s poll=%d count=%d", code, etag, poll, len(events))

	if err != nil {
		// no new events
		if code == 304 {
			return []Event{}, etag, poll, "", nil
		}

		return nil, "", -1, "", err
	}

	var next = ""
	url, _ := res.MediaHeader.Relations.Rel("next", nil)

	if url != nil {
		next = url.String()
	}

	return events, etag, poll, next, nil
}

func loadFileAuth() *auth {
	f, err := os.Open(configFile)

	if err != nil {
		warnf("Load file [%s] err: %s", configFile, err)
		return nil
	}

	var a auth

	if err := json.NewDecoder(f).Decode(&a); err != nil {
		warnf("File json decode err: %s", err)
		return nil
	}

	if a.Token != "" {
		return &a
	}

	warnf("Invalid auth found at %s: %+v", configFile, a)
	return nil
}

func saveFileAuth(a *auth) {
	f, err := os.Create(configFile)

	if err != nil {
		warnf("Create file [%s] err: %s", configFile, err)
		return
	}

	if err := json.NewEncoder(f).Encode(a); err != nil {
		warnf("File json encdoer err: %s", err)
		return
	}
}

func promptAuth() *auth {
	var (
		a    auth
		code string
	)

	fmt.Printf("Github Login: ")
	fmt.Scanln(&a.Login)

	pw, err := speakeasy.Ask("Github Password: ")

	if err != nil {
		warnf("speakeasy err: %s", err)
		return nil
	}

	// try no 2fa
	auths, err := getAuths(a.Login, pw, "")

	if err != nil {
		// need 2fa
		if oerr, ok := err.(*octokit.ResponseError); ok && oerr.Type == octokit.ErrorOneTimePasswordRequired {
			code, err = speakeasy.Ask("Github 2FA Code: ")

			if err != nil {
				warnf("speakeasy err: %s", err)
				return nil
			}

			auths, err = getAuths(a.Login, pw, code)

			if err != nil {
				warnf("get auth err: %s", err)
				return nil
			}
		} else {
			warnf("get auth err: %s", err)
			return nil
		}
	}

	for _, auth := range auths {
		if auth.Note == "github-watch" {
			a.Token = auth.Token
			return &a
		}
	}

	auth, err := createAuth(a.Login, pw, code)

	if err != nil {
		warnf("create err: %s", err)
		return nil
	}

	a.Token = auth.Token

	return &a
}

func getAuths(username string, pw string, code string) ([]octokit.Authorization, error) {
	url, err := octokit.AuthorizationsURL.Expand(nil)

	if err != nil {
		return nil, err
	}

	auth := octokit.BasicAuth{Login: username, Password: pw, OneTimePassword: code}
	cl := octokit.NewClient(auth)

	auths, res := cl.Authorizations(url).All()

	if res.HasError() {
		return nil, res.Err
	}

	return auths, nil
}

func createAuth(username string, pw string, code string) (*octokit.Authorization, error) {
	url, err := octokit.AuthorizationsURL.Expand(nil)

	if err != nil {
		return nil, err
	}

	auth := octokit.BasicAuth{Login: username, Password: pw, OneTimePassword: code}
	cl := octokit.NewClient(auth)

	p := octokit.AuthorizationParams{
		Scopes: []string{"repo"},
		Note:   "github-watch",
	}

	a, res := cl.Authorizations(url).Create(p)

	if res.HasError() {
		return nil, res.Err
	}

	return a, nil
}

func findToken(auths []octokit.Authorization) string {
	for _, a := range auths {
		if a.Note == "github-watch" {
			return a.Token
		}
	}

	return ""
}
