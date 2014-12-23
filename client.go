package main

import (
	"strconv"
	"time"

	"github.com/octokit/go-octokit/octokit"
)

type client struct {
	token string
}

type pollMsg struct {
	url    string
	events []Event
	err    error
}

func (c *client) getUser() (*octokit.User, error) {
	url, err := octokit.CurrentUserURL.Expand(nil)

	if err != nil {
		return nil, err
	}

	auth := octokit.TokenAuth{c.token}
	cl := octokit.NewClient(auth)

	u, res := cl.Users(url).One()

	if res.HasError() {
		return nil, res.Err
	}

	return u, nil
}

func (c *client) pollEvents(u string, count int, send chan *pollMsg) {
	var (
		etag   string
		lastId string
		events []Event
		err    error
		poll   = 30
	)

	go func() {
		for {
			events, etag, poll, err = c.getEventsSince(u, etag, lastId, count)

			if len(events) > 0 {
				lastId = events[0].Id
			}

			send <- &pollMsg{u, events, err}

			time.Sleep(time.Duration(poll) * time.Second)
		}
	}()
}

func (c *client) getEventsSince(u, etag, lastId string, count int) ([]Event, string, int, error) {
	var (
		events = []Event{}
		ev     []Event
		poll   int
		err    error
		next   string
	)

	for {
		ev, etag, poll, next, err = c.getEvents(u, etag)

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

		if len(events) >= count {
			events = events[0:count]
			break
		}

		if next == "" {
			break
		}

		u = next
	}

	return events, etag, poll, nil
}

func (c *client) getEvents(u string, etag string) ([]Event, string, int, string, error) {
	var (
		auth = octokit.TokenAuth{c.token}
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
