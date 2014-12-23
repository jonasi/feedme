package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/buger/goterm"
)

var (
	configFile        string
	defaultConfigFile = path.Join(os.Getenv("HOME"), ".config", "github-watch")
	debug             = flag.Bool("debug", false, "")
	orgs              = stringsl{}
	users             = stringsl{}
	userOrgs          = stringsl{}
	repos             = stringsl{}
)

func main() {
	flag.StringVar(&configFile, "config", "", "")
	flag.Var(&orgs, "org", "")
	flag.Var(&userOrgs, "user-org", "")
	flag.Var(&repos, "repo", "")
	flag.Var(&users, "user", "")
	flag.Parse()

	if configFile == "" {
		configFile = defaultConfigFile
	}

	a := loadFileAuth()

	if a == nil {
		a = promptAuth()
	}

	if a == nil {
		fmt.Println("Could not get an authorization token from Github")
		os.Exit(1)
	}

	saveFileAuth(a)

	cl := client{a.Token}

	u, err := cl.getUser()

	if err != nil {
		fmt.Printf("User load error: %s\n", err)
		os.Exit(1)
	}

	var (
		ch       = make(chan *pollMsg)
		watching = 0
	)

	for _, org := range orgs {
		cl.pollEvents(fmt.Sprintf("/orgs/%s/events", org), ch)
		watching++
	}

	for _, org := range userOrgs {
		cl.pollEvents(fmt.Sprintf("/users/%s/events/orgs/%s", u.Login, org), ch)
		watching++
	}

	for _, repo := range repos {
		cl.pollEvents(fmt.Sprintf("/repos/%s/events", repo), ch)
		watching++
	}

	for _, u := range users {
		cl.pollEvents(fmt.Sprintf("/users/%s/events", u), ch)
		watching++
	}

	if watching == 0 {
		cl.pollEvents(fmt.Sprintf("/users/%s/received_events", u.Login), ch)
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
