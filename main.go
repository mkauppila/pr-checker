package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"sync"
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

type PullRequestStatus int

const (
	Draft PullRequestStatus = iota
	Open
)

type PullRequest struct {
	title     string
	by        string
	link      string
	state     PullRequestStatus
	updatedAt time.Time
}

type LatestUpdatedAndStatus []PullRequest

func (l LatestUpdatedAndStatus) Len() int { return len(l) }

func (l LatestUpdatedAndStatus) Less(i, j int) bool {
	return l[i].updatedAt.Before(l[j].updatedAt) && l[i].state > l[j].state
}

func (l LatestUpdatedAndStatus) Swap(i, j int) { l[i], l[j] = l[j], l[i] }

type Repository struct {
	RepoName string
	Pulls    []PullRequest
}

func main() {
	var (
		accessToken string
		orgName     string
		ugly        bool
	)
	flag.StringVar(&accessToken, "token", "", "Github access token")
	flag.StringVar(&orgName, "org-name", "", "Organization name")
	flag.BoolVar(&ugly, "be-ugly", false, "Use no color, nothing fancy mode")
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
	}
	if !hasAccess {
		fmt.Println("No access to org: ", orgName)
		os.Exit(1)
	}

	repos, _, err := client.Repositories.ListByOrg(ctx, orgName, nil)
	if err != nil {
		panic(err)
	}

	var wwg sync.WaitGroup
	channel := make(chan Repository)

	for _, repo := range repos {
		wwg.Add(1)

		// AP: how does goroutine access the surrounding variables?
		go func(wg *sync.WaitGroup, name string) {
			defer wg.Done()
			// fmt.Println(name + "\n")
			pulls, _, err := client.PullRequests.List(ctx, orgName, name, nil)
			if err != nil {
				panic(err)
			}

			var prs []PullRequest
			for _, pull := range pulls {
				state := Open
				if *pull.Draft {
					state = Draft
				}

				prs = append(prs, PullRequest{
					title:     *pull.Title,
					by:        *pull.User.Login,
					link:      *pull.HTMLURL,
					state:     state,
					updatedAt: *pull.UpdatedAt,
				})
			}

			channel <- Repository{RepoName: name, Pulls: prs}
		}(&wwg, *repo.Name)
	}

	var rwg sync.WaitGroup
	rwg.Add(1)
	go func(wg *sync.WaitGroup, channel <-chan Repository) {
		defer wg.Done()

		var repos []Repository
		for repo := range channel {
			repos = append(repos, repo)
		}

		for _, r := range repos {
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

			const (
				reset = "\033[0m"
				bold  = "\033[1m"
				green = "\033[32m"
				white = "\033[37m"
			)

			if ugly {
				fmt.Println(r.RepoName)
			} else {
				fmt.Printf("%s%s%s\n", bold, r.RepoName, reset)
			}
			sort.Sort(LatestUpdatedAndStatus(r.Pulls))
			for _, pull := range r.Pulls {
				if !isFreshPullRequest(pull) {
					continue
				}

				if ugly {
					status := "Ready"
					if pull.state == Draft {
						status = "Draft"

					}
					fmt.Printf("  %s\t- %s => %s (%s)\n", status, pull.by, pull.title, pull.link)
					continue
				}

				fmt.Print("  ")
				if pull.state == Draft {
					fmt.Printf("%sDraft%s", white, reset)
				} else {
					fmt.Printf("%sReady%s", green, reset)
				}
				fmt.Printf("\t- %s => ", pull.by)
				fmt.Printf("\033]8;;%s\a%s\033]8;;\a", pull.link, pull.title)
				fmt.Printf("\n")
			}
		}
	}(&rwg, channel)

	wwg.Wait()
	close(channel)
	rwg.Wait()
}

func isFreshPullRequest(pr PullRequest) bool {
	return pr.updatedAt.After(time.Now().AddDate(0, 0, -14))
}
