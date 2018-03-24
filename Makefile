.DEFAULT_GOAL := help

APP_NAME         = technical-on-boarding
APP_PACKAGE      = github.com/samsung-cnct/container-technical-on-boarding
APP_PATH         = ./app
APP_PATH_PKGS    = $(APP_PATH)/models $(APP_PATH)/controllers $(APP_PATH)/jobs $(APP_PATH)/jobs/onboarding

# The version and build is statically set if you cannot calculate it via git.
# Additionally if APP_VERSION or APP_BUILD is overriden (?=) then these
# values will have precedence.
GIT_VERSION      = $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')
APP_VERSION     ?= $(if $(GIT_VERSION),$(GIT_VERSION),0.0.0)
GIT_HASH         = $(shell git rev-parse --short HEAD)
APP_BUILD       ?= $(if $(GIT_HASH),$(GIT_HASH),000)

IMAGE_REPO       = quay.io
IMAGE_REPO_ORG   = samsung_cnct
IMAGE_TAG       ?= local-dev
IMAGE_NAME      ?= $(IMAGE_REPO)/$(IMAGE_REPO_ORG)/$(APP_NAME):$(IMAGE_TAG)

LDFLAGS=-ldflags "-X main.Version=${APP_VERSION} -X main.Build=${APP_BUILD}"

DOCKER_RUN_PORT ?=9000
DOCKER_RUN_OPTS  =--rm -it -p ${DOCKER_RUN_PORT}:9000 --env-file ./.env
DOCKER_RUN_CMD  ?=

# Build Tools

# resolves to v2.0.3 as of 2018-01-10
GOMETALINTER := ${GOPATH}/bin/gometalinter.v2
$(GOMETALINTER):
	go get -u gopkg.in/alecthomas/gometalinter.v2

DEP := ${GOPATH}/bin/dep
$(DEP):
	go get -u github.com/golang/dep/cmd/dep

# manually pinning to v0.17
REVEL := ${GOPATH}/bin/revel
$(REVEL):
	go get -d github.com/revel/cmd/revel
	cd ${GOPATH}/src/github.com/revel/cmd; git checkout -q v0.17
	cd ${GOPATH}/src/github.com/revel/revel; git checkout -q release/v0.17
	go install github.com/revel/cmd/revel
	$(REVEL) version

.PHONY: all
all: clean vendor lint build test ## Run all targets (clean, vendor, lint, build, test)

.PHONY: vendor
vendor: $(DEP) Gopkg.lock Gopkg.toml
	$(DEP) ensure
	@touch $@

.PHONY: lint
lint: $(GOMETALINTER) ## Run lint tools (vet,gofmt,golint,gosimple)
	$(GOMETALINTER) --install
	$(GOMETALINTER) --vendored-linters \
		--disable-all \
		--enable=vet \
		--enable=gofmt \
		--enable=golint \
		--enable=gosimple \
		--sort=path \
		--aggregate \
		--vendor \
		--tests \
		$(APP_PATH)/...

.PHONY: build
build: $(APP_NAME) $(REVEL) ## Build the Go app

$(APP_NAME):
	go build -v $(LDFLAGS) $(APP_PATH_PKGS)

.PHONY: test
test: lint ## Run unit tests
	go test -race -v $(APP_PATH_PKGS)

coverage.html: $(shell find $(APP_PATH_PKGS) -name '*.go')
	go test -covermode=count -coverprofile=coverage.prof $(APP_PATH_PKGS)
	go tool cover -html=coverage.prof -o $@

.PHONY: test-cover
test-cover: coverage.html ## Run test coverage

.PHONY: clean
clean: $(REVEL) ## Clean test artifacts
	-rm -vf ./coverage.* ./$(APP_NAME)
	-rm -rf ./test-results/
	revel clean $(APP_PACKAGE)

godoc.txt: $(shell find ./ -name '*.go')
	godoc $(APP_PATH) > $@

.PHONY: docs
docs: godoc.txt ## Generate package documentation

docker-build: Dockerfile ## Build Docker image
	docker build --pull --force-rm \
	   --build-arg VERSION=$(APP_VERSION) \
	   --build-arg BUILD=$(APP_BUILD) \
	   -t $(IMAGE_NAME) .
	touch $@

.PHONY: docker-test
docker-test: docker-build ## Run functional test in container
	docker run --rm --env-file ./template.env \
		 $(IMAGE_NAME) \
		 revel test $(APP_PACKAGE) dev

.PHONY: docker-run
docker-run: docker-build ## Run container
	docker run $(DOCKER_RUN_OPTS) $(IMAGE_NAME) $(DOCKER_RUN_CMD)

.PHONY: docker-run-dev
docker-run-dev: docker-build ## Run container from local directory
	docker run $(DOCKER_RUN_OPTS) \
	   -v $(PWD):/go/src/$(APP_PACKAGE) \
		 -e VERSION=$(APP_VERSION) \
		 -e BUILD=$(APP_BUILD) \
	   $(IMAGE_NAME) $(DOCKER_RUN_CMD)

.PHONY: docker-clean
docker-clean: ## Clean docker image
	rm docker-build
	docker rmi $(IMAGE_NAME)

.PHONY: help
help: ## Display make tasks
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
