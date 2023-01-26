![Baton Logo](./docs/images/baton-logo.png)

# `baton-confluence` [![Go Reference](https://pkg.go.dev/badge/github.com/conductorone/baton-confluence.svg)](https://pkg.go.dev/github.com/conductorone/baton-confluence) ![main ci](https://github.com/conductorone/baton-confluence/actions/workflows/main.yaml/badge.svg)

`baton-confluence` is a connector for Confluence built using the [Baton SDK](https://github.com/conductorone/baton-sdk). It communicates with the confluence API to sync data about groups, and users.

Check out [Baton](https://github.com/conductorone/baton) to learn more the project in general.

# Getting Started

## brew

```
brew install conductorone/baton/baton conductorone/baton/baton-confluence
baton-confluence
baton resources
```

## docker

```
docker run --rm -v $(pwd):/out -e BATON_DOMAIN_URL=domain_url -e BATON_API_KEY=apiKey -e BATON_USERNAME=username ghcr.io/conductorone/baton-confluence:latest -f "/out/sync.c1z"
docker run --rm -v $(pwd):/out ghcr.io/conductorone/baton:latest -f "/out/sync.c1z" resources
```

## source

```
go install github.com/conductorone/baton/cmd/baton@main
go install github.com/conductorone/baton-confluence/cmd/baton-confluence@main

BATON_API_KEY=apiKey BATON_DOMAIN_URL=domainUrl BATON_USERNAME=username
baton resources
```

# Data Model

`baton-confluence` will pull down information about the following Confluence resources:
- Groups
- Users

# Contributing, Support and Issues

We started Baton because we were tired of taking screenshots and manually building spreadsheets. We welcome contributions, and ideas, no matter how small -- our goal is to make identity and permissions sprawl less painful for everyone. If you have questions, problems, or ideas: Please open a Github Issue!

See [CONTRIBUTING.md](https://github.com/ConductorOne/baton/blob/main/CONTRIBUTING.md) for more details.

# `baton-confluence` Command Line Usage

```
baton-confluence

Usage:
  baton-confluence [flags]
  baton-confluence [command]

Available Commands:
  completion         Generate the autocompletion script for the specified shell
  help               Help about any command

Flags:
  -f, --file string                         The path to the c1z file to sync with ($BATON_FILE) (default "sync.c1z")
      --api-key string                      The api key for the confluence account. ($BATON_API_KEY)
      --domain-url string                   The domain url for the confluence account. ($BATON_DOMAIN_URL)
      --username string                     The username for your Confluence account. ($BATON_USERNAME)
  -h, --help                                help for baton-confluence
      --log-format string                   The output format for logs: json, console ($BATON_LOG_FORMAT) (default "json")
      --log-level string                    The log level: debug, info, warn, error ($BATON_LOG_LEVEL) (default "info")
  -v, --version                             version for baton-confluence

Use "baton-confluence [command] --help" for more information about a command.

```
