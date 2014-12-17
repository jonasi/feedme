package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/bgentry/speakeasy"
	"github.com/octokit/go-octokit/octokit"
)

var (
	configFile = path.Join(os.Getenv("HOME"), ".config", "github-watch")
	debug      = flag.Bool("debug", false, "")
	org        = flag.String("org", "", "")
)

type auth struct {
	Login string `json:"login"`
	Token string `json:"token"`
}

type summarizable interface {
	Summary(*Event) string
}

func main() {
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

	var (
		etag string
		poll = 30
	)

	for {
		events, et, p, err := loadEvents(u.Login, *org, a, etag)
		etag = et

		if p > 0 {
			poll = p
		}

		if err != nil {
			fmt.Printf("Events load error: %s\n", err)
			os.Exit(1)
		}

		for i := len(events) - 1; i > 0; i-- {
			ev := events[i]
			var sum string

			if s, ok := ev.Payload.(summarizable); ok {
				sum = s.Summary(&ev)
			} else {
				sum = "Unhandled event [" + ev.Type + "]"
			}

			fmt.Printf("%-20s %s\n", ev.CreatedAt.Format("Jan 2 3:04:05 PM"), sum)
		}

		time.Sleep(time.Duration(poll) * time.Second)
	}
}

func warnf(str string, params ...interface{}) {
	if !*debug {
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

func loadEvents(user string, org string, a *auth, etag string) ([]Event, string, int, error) {
	var (
		u    string
		auth = octokit.TokenAuth{a.Token}
		cl   = octokit.NewClient(auth)
	)

	if org == "" {
		u = fmt.Sprintf("/users/%s/events", user)
	} else {
		u = fmt.Sprintf("/users/%s/events/orgs/%s", user, org)
	}

	req, err := cl.NewRequest(u)

	if err != nil {
		return nil, "", -1, err
	}

	if etag != "" {
		req.Header.Add("If-None-Match", etag)
	}

	events := []Event{}

	res, err := req.Get(&events)

	etag = res.Header.Get("Etag")
	poll, _ := strconv.Atoi(res.Header.Get("X-Poll-Interval"))

	if err != nil {
		// no new events
		if res.StatusCode == 304 {
			return []Event{}, etag, poll, nil
		}

		return nil, "", -1, err
	}

	return events, etag, poll, nil
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
