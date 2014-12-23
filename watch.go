package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"sort"
)

var (
	configFile        string
	defaultConfigFile = path.Join(os.Getenv("HOME"), ".config", "feedme")
	debug             bool
	count             int
	tail              bool
	orgs              = stringsl{}
	users             = stringsl{}
	userOrgs          = stringsl{}
	repos             = stringsl{}
)

func main() {
	flag.StringVar(&configFile, "config", "", "")
	flag.IntVar(&count, "n", 30, "")
	flag.BoolVar(&debug, "debug", false, "")
	flag.BoolVar(&tail, "f", false, "")
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
		ch      = make(chan *pollMsg)
		urls    = []string{}
		polling = map[string]int{}
	)

	for _, org := range orgs {
		urls = append(urls, fmt.Sprintf("/orgs/%s/events", org))
	}

	for _, org := range userOrgs {
		urls = append(urls, fmt.Sprintf("/users/%s/events/orgs/%s", u.Login, org))
	}

	for _, repo := range repos {
		urls = append(urls, fmt.Sprintf("/repos/%s/events", repo))
	}

	for _, u := range users {
		urls = append(urls, fmt.Sprintf("/users/%s/events", u))
	}

	if len(urls) == 0 {
		urls = append(urls, fmt.Sprintf("/users/%s/received_events", u.Login))
	}

	for _, u := range urls {
		if _, ok := polling[u]; ok {
			continue
		}

		polling[u] = 0
		cl.pollEvents(u, count, ch)
	}

	var (
		first  = false
		events = events{}
	)

	for msg := range ch {
		polling[msg.url]++

		if !first {
			first = true

			for _, c := range polling {
				if c == 0 {
					first = false
					break
				}
			}
		}

		if msg.err != nil {
			fmt.Printf("Events load error: %s\n", msg.err)
			continue
		}

		events = append(events, msg.events...)

		if first {
			sort.Sort(events)

			for _, ev := range events {
				printEvent(&ev)
			}

			events = nil

			if !tail {
				os.Exit(0)
			}
		}
	}
}

func printEvent(ev *Event) {
	fmt.Println("\n" + ev.Summary())
}

type events []Event

func (e events) Len() int {
	return len(e)
}

func (e events) Less(i, j int) bool {
	return e[i].CreatedAt.Before(e[j].CreatedAt)
}

func (e events) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}
