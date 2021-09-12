# Pull Request Checker

Check all active pull requests from a Github organization with one command.

The output of the tool looks something like below in the default mode. It lists all the repositories for the organisation and all active PRs for all the repositories. Pull request's title doubles as a link to it.

```
Backend
  Ready	- mkauppila => Add new endpoints for the user profile
Frontend
  Ready	- LeChuck => Redo back button logic for forms
  Draft	- mkauppila => New profile view
Infra
  Draft	- Guybrush => [PoC] Testing out AWS Aurora
```

## How install

Simple way:

-   [Download the macOS binary from releases](https://github.com/mkauppila/pr-checker/releases/tag/release-1.00-macos)

With [go tooling](https://golang.org) installed:

-   run `go build`

Copy the downloaded (or freshly built) binary to `/usr/local/bin/`.

## Run and configure

The PR checker supports a config file for more convenient usage.
The location needs to be `$HOME/.pr-checker/config.json` and here's and example
contents for it:

```
{
  "AccessToken": "some-access-token-here",
  "OrgName": "the organization name here"
}
```

The access token is a [Github personal access token](https://github.com/settings/tokens). It should have as little as possible permissions to Github. This set of permissions work: `read:enterprise, read:org, read:user, repo` but
it might not be the smallest set of permissions needed.

The same information can be given as command line options when needed. See `pr-checker --help` for more information.

PR Checker uses colors and links for formatting the output which requires support from the terminal. Try `--be-ugly` command line option to see the plain output formatting without anything fancy.

## License

Copyright © 2021 Markus Kauppila. Licensed under MIT license.
