package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/google/go-github/v39/github"
	"golang.org/x/oauth2"
)

func authenticatedGithubClient(accessToken string) *github.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	tc := oauth2.NewClient(context.Background(), ts)

	return github.NewClient(tc)
}

type PullRequest struct {
	title     string
	by        string
	link      string
	state     string
	updatedAt time.Time
}

type Repository struct {
	RepoName string
	Pulls    []PullRequest
}

func main() {
	var (
		accessToken string
		orgName     string
	)
	flag.StringVar(&accessToken, "token", "", "Github access token")
	flag.StringVar(&orgName, "org-name", "", "Organization name")
	flag.Parse()

	if len(accessToken) == 0 || len(orgName) == 0 {
		fmt.Println("Missing command line args")
		fmt.Println("Run: pr-checker --help")
		os.Exit(0)
	}

	client := authenticatedGithubClient(accessToken)

	ctx := context.Background()

	orgs, _, _ := client.Organizations.ListOrgMemberships(ctx, nil)
	hasAccess := false
	for _, org := range orgs {
		if *org.Organization.Login == orgName {
			hasAccess = true
			break
		}
		b, _ := json.MarshalIndent(org, "", " ")
		fmt.Println(string(b) + "\n")
	}
	if hasAccess {
		fmt.Println("Access to org: ", orgName)
	} else {
		fmt.Println("No access to org: ", orgName)
		os.Exit(1)
	}

	repos, _, err := client.Repositories.ListByOrg(ctx, orgName, nil)
	if err != nil {
		panic(err)
	}

	var repoPrs []Repository
	for _, repo := range repos {
		fmt.Println(*repo.Name + "\n")
		pulls, _, err := client.PullRequests.List(ctx, orgName, *repo.Name, nil)
		if err != nil {
			panic(err)
		}

		var prs []PullRequest
		for _, pull := range pulls {
			state := "OK"
			if *pull.Draft {
				state = "Draft"
			}

			prs = append(prs, PullRequest{
				title:     *pull.Title,
				by:        *pull.User.Login,
				link:      *pull.HTMLURL,
				state:     state,
				updatedAt: *pull.UpdatedAt,
			})
		}

		t := Repository{RepoName: *repo.Name, Pulls: prs}
		repoPrs = append(repoPrs, t)
	}

	for _, r := range repoPrs {
		hasFreshPrs := false
		for _, pull := range r.Pulls {
			if isFreshPullRequest(pull) {
				hasFreshPrs = true
				break
			}
		}
		if !hasFreshPrs || len(r.Pulls) == 0 {
			continue
		}

		fmt.Println(r.RepoName)
		for _, pull := range r.Pulls {
			if !isFreshPullRequest(pull) {
				continue
			}

			fmt.Println("  ", pull.state+" - "+pull.by+" -> "+pull.title)
			fmt.Println("    ", pull.link)
		}
	}
}

func isFreshPullRequest(pr PullRequest) bool {
	return pr.updatedAt.After(time.Now().AddDate(0, 0, -14))
}
