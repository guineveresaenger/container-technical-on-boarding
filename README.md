# Samsung CNCT Technical On Boarding container
[![pipeline status](https://git.cnct.io/common-tools/samsung-cnct_container-technical-on-boarding/badges/master/pipeline.svg)](https://git.cnct.io/common-tools/samsung-cnct_container-technical-on-boarding/commits/master)
[![Docker Repository on Quay](https://quay.io/repository/samsung_cnct/technical-on-boarding-container/status "Docker Repository on Quay")](https://quay.io/repository/samsung_cnct/technical-on-boarding-container)

This repo contains a Dockerfile to build and test the technical on-boarding container.

This container is used by the chart-technical-on-boarding repo, and is part of the on-boarding process for new Samsung CNCT employees.

## Goals

- Automate the preparation of a new CNCT employee's on-boarding, as a list of tasks, represented as issues in GitHub.
- Provide a functional example of a Go program, integrating with the GitHub API.

### Functional Requirements

- Loads a template of "tasks" to be assigned to a new-hire.
- Creates a Milestone and Project in GitHub.
- Creates Issues in GitHub to represent tasks, and links them to Milestone and Project.
- Assigns those Issues to the new-hire.

## Usage

To generate the on boarding tasks go [here](http://technical-on-boarding.cnct.io) and
follow the instructions. To run a local instance for the purpose of examining and
experimenting with the source code see [below](#development-and-testing).

## Development and Testing

### Running in Container

Before you start you'll need:

- Docker
- Make
- A local environment file

To make an environment file that contains the Github client credentials you can copy the `template.env`
file like so:

```shell
cp template.env .env
```

To build and run the application in a container execute the following:

```shell
make docker-run
```

This will start a local HTTP server at [127.0.0.1:9000](http://127.0.0.1:9000/). Open this URL
in your browser, click the *Authorize* button, and log into GitHub _as yourself_. Once authenticated,
the application will direct you to the workload screen, start the task generating job, and
provide progress on the job.

To support [Revel's][4] _hot code reload_ in container, use the `run-dev` option to map
your working directory into the container like so:

```shell
make docker-run-dev
```

#### Alternate listening IP addresses.

*NOTE*: In some Docker configurations this may listen on a different IP address.
Typically this is most easily found via:

```shell
env | grep DOCKER_HOST  # hopefully not empty...
```

Or potentially:

```shell
docker run --net=host codenvy/che-ip
```

If either of the above indicate another IP address, try reaching that on port 9000.

### Modifying task lists

The tasks are loaded in from a [top-level yaml file](./onboarding-issues.yaml). You can configure these to your own wishes. Things to note:
- You may modify the `title` and `description` fields for each task. The `assignee` field should remain the same for all tasks.
- The `tags` field refers to the learning track that each task belongs to. You may devise your own track, e.g. `Learn German`. You would create a task with appropriate content and title, and add `Learn German` to the `tags` entry as a list item, and when the app runs, one of the tracks offered on the website will be "Learn German" and include that task. Tasks can have multiple tags, which you might choose to do if their contents are a requirement for more than one track.

#### Custom task list with CNCT onboarding chart

There is a [helm chart](https://github.com/samsung-cnct/chart-technical-on-boarding) that deploys the official CNCT onboarding app. Its default container image is hosted in CNCT's [Quay](http://quay.io) container registry. To use your own container, follow these steps:

1. In the container-technical-onboarding directory, run:
`IMAGE_NAME=<your_custom_container>:<local-dev> make docker-build`
2. Host the resulting container image in a container registry such as DockerHub or Quay
3. Follow [instructions](https://github.com/samsung-cnct/chart-technical-on-boarding/README.md#custom-task-board) in the onboarding helm chart to edit the `values.yaml` file to have the `image` and `tag` fields point at your container registry link.





### Running Locally (in your desktop)

If you prefer to build and run this in Go on your desktop environment,
rather than a preconfigured Docker container, here's how...

Please ensure that [Go is properly set up](./SETTINGUPGO.md) first.

```shell
make all

# app credentials required
ONBOARD_CLIENT_ID="{clientid}" \
    ONBOARD_CLIENT_SECRET="{clientsecret}" \
    ONBOARD_TASKS_FILE="./onboarding-issues.yaml" \
    ONBOARD_REPO="technical-on-boarding" \
    ONBOARD_ORG="samsung-cnct" \
    VERSION="1.1.3" \
    BUILD="local" \
    revel run github.com/samsung-cnct/container-technical-on-boarding
```

*NOTE*: You can get help for the Makefile with `make help`.

This workload relies heavily on the GitHub API, which also requires valid application tokens.

To facilitate testing, this project includes a fairly robust mock of the GitHub API client, 
and relies on interfaces and proxy methods in several other points to allow the business 
logic to operate against a local testing environment without reaching GitHub's API service.

Per Golang's convention, tests are found in files ending with `_test.go`.

[2]: https://github.com/settings/applications/new
[3]: https://github.com/settings/apps
[4]: https://revel.github.io://revel.github.io/manual/debug.html "Revel Hot Code Reload"
