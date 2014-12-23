package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bgentry/speakeasy"
	"github.com/octokit/go-octokit/octokit"
)

type auth struct {
	Login string `json:"login"`
	Token string `json:"token"`
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
