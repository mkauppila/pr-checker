package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
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

type Config struct {
	AccessToken string
	OrgName     string
	Ugly        bool
}

func configFileExists() (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, fmt.Errorf("no user home directory. Error: %s", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".pr-checker", "config.json")); err != nil {
		return false, err
	}

	return true, nil
}

func readConfigFile() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("no user home directory. Error: %s", err)
	}
	// We know the file exists so skip the error
	file, _ := os.ReadFile(filepath.Join(home, ".pr-checker", "config.json"))

	var config Config
	// read the confs from here and that's it!
	if err := json.Unmarshal(file, &config); err != nil {
		return Config{}, fmt.Errorf("failed to parse config file. Error: %s", err)
	}

	return config, nil
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

	configExists, err := configFileExists()
	if err != nil {
		fmt.Println("Issue with config. Error: ", err)
		os.Exit(1)
	}

	if configExists {
		conf, err := readConfigFile()
		if err != nil {
			fmt.Println("Config.json found with error: ", err)
			os.Exit(1)
		}
		if accessToken == "" {
			accessToken = conf.AccessToken
		}
		if orgName == "" {
			orgName = conf.OrgName
		}
		// FIXME: does't work together with the config file override
		if !ugly {
			ugly = conf.Ugly
		}
	}

	if len(accessToken) == 0 || len(orgName) == 0 {
		fmt.Println("Missing command line args or a config file at `$HOME/.pr-checker/config.json")
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
