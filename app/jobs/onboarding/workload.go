/*
This package implements some fairly thin wrappers over the GitHub client API, to allow them to be mocked for testing.
*/

package onboarding

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/google/go-github/github"
	"github.com/samsung-cnct/container-technical-on-boarding/app/jobs"
)

type (

	// WorkflowClient interfaces with GitHub's Client model, providing access to repositories, and supporting a mock interface.
	WorkflowClient struct {
		Context context.Context
		Client  IGitHubClient
	}

	// WorkflowRepository provides derived access to repository-scoped resources, e.g. issues, projects, etc.
	WorkflowRepository struct {
		Client  IGitHubClient
		Context context.Context
		*github.Repository
	}

	// GitHubClientWrapper implements iGitHubClient, and provides a this abstraction to simplify mocking for test.
	GitHubClientWrapper struct {
		*github.Client
	}

	// IGitHubClient implemented by GitHubClientWrapper, abstracts properties of *github.Client
	IGitHubClient interface {
		getIssuesService() iGitHubIssues
		getRepositoriesService() iGitHubRepositories
		getProjectsService() iGitHubProjects
		getUsersService() iGitHubUsers
	}

	iGitHubUsers interface {
		// for github.Client.Users
		Get(ctx context.Context, username string) (*github.User, *github.Response, error)
	}

	iGitHubIssues interface {
		// for github.Client.Issues
		ListMilestones(ctx context.Context, owner string, repo string, opts *github.MilestoneListOptions) ([]*github.Milestone, *github.Response, error)
		CreateMilestone(ctx context.Context, owner string, repo string, opts *github.Milestone) (*github.Milestone, *github.Response, error)
		ListByRepo(ctx context.Context, owner string, repo string, opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error)
		Create(ctx context.Context, owner string, repo string, req *github.IssueRequest) (*github.Issue, *github.Response, error)
		Edit(ctx context.Context, owner string, repo string, issueID int, req *github.IssueRequest) (*github.Issue, *github.Response, error)
	}

	iGitHubRepositories interface {
		// for github.Client.Repositories
		CreateProject(ctx context.Context, owner string, repo string, opts *github.ProjectOptions) (*github.Project, *github.Response, error)
		ListProjects(ctx context.Context, owner string, repo string, opts *github.ProjectListOptions) ([]*github.Project, *github.Response, error)
		Get(ctx context.Context, owner string, repo string) (*github.Repository, *github.Response, error)
	}

	iGitHubProjects interface {
		// for github.Client.Projects
		UpdateProject(ctx context.Context, projectID int, opts *github.ProjectOptions) (*github.Project, *github.Response, error)
		CreateProjectColumn(ctx context.Context, projectID int, opts *github.ProjectColumnOptions) (*github.ProjectColumn, *github.Response, error)
		ListProjectColumns(ctx context.Context, projectID int, opts *github.ListOptions) ([]*github.ProjectColumn, *github.Response, error)
		CreateProjectCard(ctx context.Context, columnID int, opt *github.ProjectCardOptions) (*github.ProjectCard, *github.Response, error)
		ListProjectCards(ctx context.Context, columnID int, opt *github.ListOptions) ([]*github.ProjectCard, *github.Response, error)
	}

	// IRepositoryAccess provides simplified procedures for this project's business case, namily masking non-idempotent requests to reduce duplication.
	IRepositoryAccess interface {
		// Methods implemented in our proxy
		GetIssuesByRequest(request *github.IssueRequest) ([]*github.Issue, error)
		CreateOrUpdateIssue(assignee *string, title *string, body *string, milestone int) (*github.Issue, error)
		CreateOrUpdateMilestone(title *string, description *string, dueDate *time.Time) (*github.Milestone, error)
		CreateOrUpdateProject(title *string, description *string, columns []string) (*github.Project, error)
		FetchMappedProjectColumns(project *github.Project) (map[string](*github.ProjectColumn), error)
		ColumnsPresent(project *github.Project, columns []string) (bool, error)
		CreateCardForIssue(issue *github.Issue, column *github.ProjectColumn) (*github.ProjectCard, error)

		// Internal methods (to be overridden by test models)
		createIssue(service iGitHubIssues, req *github.IssueRequest) (*github.Issue, error)
		updateIssue(service iGitHubIssues, issue *github.Issue, req *github.IssueRequest) (*github.Issue, error)
		fetchIssues(service iGitHubIssues, listOpts *github.IssueListByRepoOptions) ([](*github.Issue), error)
		fetchProjects(service iGitHubRepositories, listOpts *github.ProjectListOptions) ([]*github.Project, error)
		createProject(service iGitHubRepositories, createOpts *github.ProjectOptions) (*github.Project, error)
		updateProject(service iGitHubProjects, project *github.Project, updateOpts *github.ProjectOptions) (*github.Project, error)
		fetchProjectColumns(service iGitHubProjects, project *github.Project) ([]*github.ProjectColumn, error)
		createProjectColumns(service iGitHubProjects, project *github.Project, columns []string) ([]*github.ProjectColumn, error)
		createMilestone(service iGitHubIssues, ms *github.Milestone) (*github.Milestone, error)
		fetchMilestones(service iGitHubIssues, listOpts *github.MilestoneListOptions) ([]*github.Milestone, error)
	}

	iClientAccess interface {
		// Methods implemented in our proxy
		GetRepository(owner string, name string) (IRepositoryAccess, error)

		// Internal methods (to be overridden by test models)
		fetchRepository(service iGitHubRepositories, owner *string, name *string) (IRepositoryAccess, error)
		resolveUser(username *string) *github.User
	}
)

// NOTE: this reflects a business process assumption.
// Target 3 weeks (rounding up) for onboarding completion, ending on Fridays.
// New hires starting on Mondays will effectively get 4 weeks.
func getMilestoneDueTime(fromTime *time.Time) time.Time {
	if fromTime == nil {
		now := time.Now()
		fromTime = &now
	}

	// NB: if `fromTime` is a Saturday, this will be (-1).
	offset := (time.Friday - fromTime.Weekday())
	return fromTime.AddDate(0, 0, 21+int(offset))
}

// GenerateProject represents a Job to be executed by the revel job module.
// See -> https://revel.github.io/modules/jobs.html#implementing-jobs
type GenerateProject struct {
	ID      int
	Setup   *SetupScheme
	AuthEnv *AuthEnvironment
	New     chan<- jobs.Event
	Tracks  []string
}

// Run implements the required cron.Job interface for revel job execution
func (job GenerateProject) Run() {
	setup := job.Setup
	auth := job.AuthEnv
	username := auth.GithubUsername()
	client, _ := auth.newWorkflowClient(setup.GitHubEndpoint)
	tracks := job.Tracks

	// TODO = read these somewhere track := job.Setup.Tasks[0].Tags

	defer close(job.New)
	job.New <- jobs.NewEvent(job.ID, "start", fmt.Sprintf("Starting project generation as %v", username))

	repo, err := client.GetRepository(setup.GithubOrganization, setup.GithubRepository)
	if err != nil {
		job.New <- jobs.NewError(job.ID, fmt.Sprintf("Failed to repository - %s", setup.GithubRepository), err.Error())
		return
	}

	isCollaborator := auth.IsCollaborator(setup.GithubOrganization, setup.GithubRepository, username)
	if !isCollaborator {
		errMessage := fmt.Sprintf("The user (%s) is not a collaborator of the repo: %s/%s",
			username, setup.GithubOrganization, setup.GithubRepository)
		job.New <- jobs.NewError(job.ID, errMessage, errMessage)
		return
	}

	hasWritePermission := auth.HasWritePermission(setup.GithubOrganization, setup.GithubRepository, username)
	if !hasWritePermission {
		errMessage := fmt.Sprintf("The user (%s) does not have write permission on the repo: %s/%s",
			username, setup.GithubOrganization, setup.GithubRepository)
		job.New <- jobs.NewError(job.ID, errMessage, errMessage)
		return
	}

	title := fmt.Sprintf("Welcome @%s!", username)
	description := fmt.Sprintf("Let's setup up @%s for success. Here's what we need to cover...", username)
	dueOn := getMilestoneDueTime(nil)

	job.New <- jobs.NewEvent(job.ID, "progress", fmt.Sprintf("Creating Milestone - %s", title))
	milestone, err := repo.CreateOrUpdateMilestone(&title, &description, &dueOn)
	if err != nil {
		job.New <- jobs.NewError(job.ID, fmt.Sprintf("Failed to create milestone - %s", title), err.Error())
		return
	}

	job.New <- jobs.NewEvent(job.ID, "progress", fmt.Sprintf("Creating Project - %s", title))
	project, err := repo.CreateOrUpdateProject(&title, &description, []string{"Backlog", "In Progress", "Review", "Done"})
	if err != nil {
		job.New <- jobs.NewError(job.ID, fmt.Sprintf("Failed to create project - %s", title), err.Error())
		return
	}

	columns, err := repo.FetchMappedProjectColumns(project)
	if err != nil {
		job.New <- jobs.NewError(job.ID, "Failed to fetch project columns", err.Error())
		return
	}
	var cards []string
	for _, task := range setup.Tasks {
		for _, tag := range task.Tags {
			fmt.Println("Hello", tag)
			if CheckTracks(tracks, tag) {
				if CardExists(cards, task.Title) {
					fmt.Println("HEEEEKWLKDLSFIDUFOOOOO")
					fmt.Println("task was created by a previous track already")
				} else {
					cards = append(cards, task.Title)
					fmt.Println("HERE ARE THE CARDS:", cards)
					job.New <- jobs.NewEvent(job.ID, "progress", fmt.Sprintf("Preparing Issue - %s", task.Title))
					issue, err := repo.CreateOrUpdateIssue(&task.Assignee.GithubUsername, &task.Title, &task.Description, milestone.GetNumber())
					if err != nil {
						job.New <- jobs.NewError(job.ID, fmt.Sprintf("Failed to create issue - %s", task.Title), err.Error())
						return
					}
					// NOTE: this fails with HTTP 422 when the the issue already has a card in the project.
					_, err = repo.CreateCardForIssue(issue, columns["Backlog"])
					if err != nil {
						job.New <- jobs.NewError(job.ID, fmt.Sprintf("Error creating card - %v", err), err.Error())
						// DO NOT return here.
					}
				}
			}
		}
	}

	//https://github.com/alika/test-toby/projects
	projectsURL := fmt.Sprintf("https://github.com/%s/%s/projects/", job.Setup.GithubOrganization, job.Setup.GithubRepository)
	completed := fmt.Sprintf("Successfully created project @ %s", projectsURL)
	job.New <- jobs.NewEvent(job.ID, "complete", completed)
}

// NewGitHubWrapper provides a simple access API over specific attributes, to support interface compatibility.
func NewGitHubWrapper(client *github.Client) IGitHubClient {
	return &GitHubClientWrapper{client}
}

func (wrap *GitHubClientWrapper) getRepositoriesService() iGitHubRepositories {
	return wrap.Client.Repositories
}

func (wrap *GitHubClientWrapper) getIssuesService() iGitHubIssues {
	return wrap.Client.Issues
}

func (wrap *GitHubClientWrapper) getProjectsService() iGitHubProjects {
	return wrap.Client.Projects
}

func (wrap *GitHubClientWrapper) getUsersService() iGitHubUsers {
	return wrap.Client.Users
}

// This method is an abstraction intended to be overridden by test models.
func (client *WorkflowClient) fetchRepository(service iGitHubRepositories, owner *string, name *string) (IRepositoryAccess, error) {

	repo, _, err := service.Get(client.Context, *owner, *name)
	if err != nil {
		return nil, fmt.Errorf("Failed fetchRepository(): %v", err)
	}
	return &WorkflowRepository{client.Client, client.Context, repo}, nil
}

// GetRepository returns a WorkflowRepository instance, wrapping the standard GitHub repository model.
func (client *WorkflowClient) GetRepository(owner string, name string) (IRepositoryAccess, error) {
	service := client.Client.getRepositoriesService()
	repo, err := client.fetchRepository(service, &owner, &name)
	return repo, err
}

// This method is an abstraction intended to be overridden by test models.
func (repo *WorkflowRepository) fetchIssues(service iGitHubIssues, listOpts *github.IssueListByRepoOptions) ([](*github.Issue), error) {

	context := repo.Context
	owner := repo.Owner.GetLogin()
	repoName := repo.GetName()

	var err error
	var resultIssues [](*github.Issue)

	// Always start with first page.
	listOpts.Page = 0

	for {
		currentIssues, response, err := service.ListByRepo(context, owner, repoName, listOpts)
		if err != nil {
			break
		}
		resultIssues = append(resultIssues, currentIssues...)
		if response.NextPage == 0 {
			break
		}
		listOpts.Page = response.NextPage
	}

	return resultIssues, err
}

// GetIssuesByRequest fetches an issue, if present, by title, milestone, and assignee username
func (repo *WorkflowRepository) GetIssuesByRequest(request *github.IssueRequest) ([]*github.Issue, error) {

	var resultIssues [](*github.Issue)

	milestone := "*"
	assignee := "*"

	requestedAssignees := request.GetAssignees()

	if (request.Milestone != nil) && (*request.Milestone) > 0 {
		milestone = strconv.Itoa(*request.Milestone)
	}

	if (len(requestedAssignees) == 1) && (requestedAssignees[0] != "none") {
		assignee = requestedAssignees[0]
	}

	listOpts := github.IssueListByRepoOptions{
		Assignee:  assignee,
		Milestone: milestone,
	}

	service := repo.Client.getIssuesService()
	issues, err := repo.fetchIssues(service, &listOpts)

	if err != nil {
		return nil, err
	}

	for _, thisIssue := range issues {
		if (request.Title == nil) || ((*request.Title) == thisIssue.GetTitle()) {
			resultIssues = append(resultIssues, thisIssue)
		}
	}

	return resultIssues, nil
}

// This method is an abstraction intended to be overridden by test models.
func (repo *WorkflowRepository) createIssue(service iGitHubIssues, req *github.IssueRequest) (*github.Issue, error) {
	owner := repo.Owner.GetLogin()
	issue, _, err := service.Create(repo.Context, owner, repo.GetName(), req)
	if err != nil {
		return nil, err
	}

	return issue, nil // success
}

// This method is an abstraction intended to be overridden by test models.
func (repo *WorkflowRepository) updateIssue(service iGitHubIssues, issue *github.Issue, req *github.IssueRequest) (*github.Issue, error) {
	owner := repo.Owner.GetLogin()
	issue, _, err := service.Edit(repo.Context, owner, repo.GetName(), issue.GetNumber(), req)
	if err != nil {
		return nil, err
	}
	return issue, nil // success
}

// CreateOrUpdateIssue searches existing issues in the repository, and returns one matching or creates a new issue.
func (repo *WorkflowRepository) CreateOrUpdateIssue(assignee *string, title *string, body *string, milestone int) (*github.Issue, error) {

	request := github.IssueRequest{}

	if assignee != nil {
		var assigneeList = []string{*assignee}
		request.Assignees = &assigneeList
	}

	if title != nil {
		request.Title = title
	}

	if body != nil {
		request.Body = body
	}

	if milestone > 0 {
		request.Milestone = &milestone
	}

	// log.Printf("Searching issues; assignee: %v; milestone: %v", *request.Assignees, *request.Milestone)

	issuesFound, err := repo.GetIssuesByRequest(&request)

	// log.Printf("Found results: %#v", issuesFound)

	if err != nil {
		return nil, err
	}

	for _, issue := range issuesFound {
		if issue != nil {
			return issue, nil // found a matching issue
		}
	}

	issue, err := repo.createIssue(repo.Client.getIssuesService(), &request)

	if err != nil {
		return nil, err
	}

	if issue.GetBody() != (*body) {
		issue, err = repo.updateIssue(repo.Client.getIssuesService(), issue, &request)
	}

	return issue, err // successfully created it (or maybe failed Edit() above)
}

// This method is an abstraction intended to be overridden by test models.
func (client *WorkflowClient) resolveUser(username *string) *github.User {
	// Alternate mode of declaration; this was initially used in debugging an interface handling issue.
	// var service IGitHubUsers = client.Client.getUsersService()

	service := client.Client.getUsersService()

	// log.Printf("Users service is: %#v", service)

	user, _, err := service.Get(client.Context, *username) // errors here

	if err != nil { // This is deemed to be fatal in our world, as assigning issues is key functionality.
		log.Fatalf("Failed to resolve user: %v", err)
		panic(err)
	}
	return user
}

// This method is an abstraction intended to be overridden by test models.
func (repo *WorkflowRepository) fetchMilestones(service iGitHubIssues, listOpts *github.MilestoneListOptions) ([]*github.Milestone, error) {
	var resultMilestones []*github.Milestone

	stop := false
	ctx := repo.Context
	owner := repo.Owner.GetLogin()
	listOpts.Page = 0

	for !stop {
		result, response, err := service.ListMilestones(ctx, owner, repo.GetName(), listOpts)
		if err != nil {
			return nil, err
		}
		resultMilestones = append(resultMilestones, result...)
		if response.NextPage == 0 {
			break
		}
		listOpts.Page = response.NextPage
	}

	return resultMilestones, nil
}

// This method is an abstraction intended to be overridden by test models.
func (repo *WorkflowRepository) createMilestone(service iGitHubIssues, ms *github.Milestone) (*github.Milestone, error) {
	owner := repo.Owner.GetLogin()
	milestone, _, err := service.CreateMilestone(repo.Context, owner, repo.GetName(), ms)
	if err != nil {
		return nil, err
	}
	return milestone, nil // success
}

// CreateOrUpdateMilestone retrieves an existing milestone by name, or creates a new one if needed.
// This operation is coupled, as GitHub will error in creating a new milestone if one already exists.
func (repo *WorkflowRepository) CreateOrUpdateMilestone(title *string, description *string, dueDate *time.Time) (*github.Milestone, error) {

	newMilestone := github.Milestone{
		Title:       title,
		Description: description,
		DueOn:       dueDate,
	}

	searchOptions := github.MilestoneListOptions{
		Sort:      "due_date",
		Direction: "desc",
	}

	availableMilestones, err := repo.fetchMilestones(repo.Client.getIssuesService(), &searchOptions)

	if err != nil {
		return nil, err
	}

	for _, ms := range availableMilestones {
		if ms.GetTitle() == *title {
			return ms, nil // found one existing that matches.
		}
	}

	milestoneCreated, err := repo.createMilestone(repo.Client.getIssuesService(), &newMilestone)
	if err != nil {
		return nil, err
	}

	return milestoneCreated, nil

}

// This method is an abstraction intended to be overridden by test models.
func (repo *WorkflowRepository) fetchProjects(service iGitHubRepositories, listOpts *github.ProjectListOptions) ([]*github.Project, error) {
	var resultProjects []*github.Project

	stop := false
	ctx := repo.Context
	owner := repo.Owner.GetLogin()
	listOpts.Page = 0

	for !stop {
		result, response, err := service.ListProjects(ctx, owner, repo.GetName(), listOpts)
		if err != nil {
			return nil, err
		}
		resultProjects = append(resultProjects, result...)
		if response.NextPage == 0 {
			break
		}
		listOpts.Page = response.NextPage
	}

	return resultProjects, nil
}

// This method is an abstraction intended to be overridden by test models.
func (repo *WorkflowRepository) createProject(service iGitHubRepositories, createOpts *github.ProjectOptions) (*github.Project, error) {
	owner := repo.Owner.GetLogin()
	proj, _, err := service.CreateProject(repo.Context, owner, repo.GetName(), createOpts)
	return proj, err
}

// This method is an abstraction intended to be overridden by test models.
func (repo *WorkflowRepository) updateProject(service iGitHubProjects, project *github.Project, updateOpts *github.ProjectOptions) (*github.Project, error) {
	proj, _, err := service.UpdateProject(repo.Context, project.GetID(), updateOpts)
	return proj, err
}

// This method is an abstraction intended to be overridden by test models.
func (repo *WorkflowRepository) createProjectColumns(service iGitHubProjects, project *github.Project, columns []string) ([]*github.ProjectColumn, error) {
	var resultColumns []*github.ProjectColumn
	ctx := repo.Context

	for _, col := range columns {
		if len(col) == 0 {
			continue
		}
		colResult, _, err := service.CreateProjectColumn(ctx, project.GetID(), &github.ProjectColumnOptions{
			Name: col,
		})
		if err != nil {
			return nil, err
		}
		resultColumns = append(resultColumns, colResult)
	}

	return resultColumns, nil // success
}

// CreateOrUpdateProject retrieves an existing GitHub Project by name, or creates a new one if needed.
// The provided column names are only used when creating a project anew.
func (repo *WorkflowRepository) CreateOrUpdateProject(title *string, description *string, columns []string) (*github.Project, error) {

	var projectFound *github.Project
	var err error
	var updateNeeded = false

	listOpts := github.ProjectListOptions{}

	createProjectOptions := github.ProjectOptions{
		Name: *title,
		Body: *description,
	}

	availableProjects, err := repo.fetchProjects(repo.Client.getRepositoriesService(), &listOpts)

	for _, proj := range availableProjects {
		if proj.GetName() == *title {
			projectFound = proj
			break
		}
	}

	if (projectFound != nil) && (projectFound.GetNumber() > 0) {

		if projectFound.GetBody() != *description {
			updateNeeded = true
		}

		if updateNeeded {
			projectFound, err = repo.updateProject(repo.Client.getProjectsService(), projectFound, &createProjectOptions)
			if err != nil {
				return nil, err
			}
		}
	}

	if (projectFound == nil) || (projectFound.GetNumber() < 1) {
		projectFound, err = repo.createProject(repo.Client.getRepositoriesService(), &createProjectOptions)

		if err != nil {
			return nil, err
		}

		_, err := repo.createProjectColumns(repo.Client.getProjectsService(), projectFound, columns)

		if err != nil {
			return nil, err
		}

	}

	return projectFound, nil
}

// This method is an abstraction intended to be overridden by test models.
func (repo *WorkflowRepository) fetchProjectColumns(service iGitHubProjects, project *github.Project) ([]*github.ProjectColumn, error) {
	var resultColumns []*github.ProjectColumn
	ctx := repo.Context
	listOpts := github.ListOptions{}

	for {
		result, response, err := service.ListProjectColumns(ctx, project.GetID(), &listOpts)
		if err != nil {
			return nil, err
		}
		resultColumns = append(resultColumns, result...)
		if response.NextPage == 0 {
			break
		}
		listOpts.Page = response.NextPage
	}

	// log.Printf("Retrieved %d columns: %#v", len(resultColumns), resultColumns)

	return resultColumns, nil
}

func (repo *WorkflowRepository) fetchProjectCards(column *github.ProjectColumn) ([]*github.ProjectCard, error) {
	var resultCards []*github.ProjectCard

	service := repo.Client.getProjectsService()
	listOpts := &github.ListOptions{Page: 0}

	for {
		result, response, err := service.ListProjectCards(repo.Context, column.GetID(), listOpts)
		if err != nil {
			return nil, err
		}
		resultCards = append(resultCards, result...)
		if response.NextPage == 0 {
			break
		}
		listOpts.Page = response.NextPage
	}

	return resultCards, nil
}

// GetFirstColumn fetches the columns associated with a GitHub Project, and returns the one with the smallest ID, assuming that this indicates it was created first.
func (repo *WorkflowRepository) GetFirstColumn(project *github.Project) (*github.ProjectColumn, error) {
	var column *github.ProjectColumn
	minColumnID := 0
	service := repo.Client.getProjectsService()
	columnsList, err := repo.fetchProjectColumns(service, project)

	if err != nil {
		return nil, err
	}

	for _, col := range columnsList {
		if (minColumnID == 0) || (minColumnID > col.GetID()) {
			minColumnID = col.GetID()
			column = col
		}
	}
	return column, nil
}

// CreateCardForIssue creates a Card in a GitHub project for a given GitHub Issue
func (repo *WorkflowRepository) CreateCardForIssue(issue *github.Issue, column *github.ProjectColumn) (*github.ProjectCard, error) {

	service := repo.Client.getProjectsService()

	cardOpts := github.ProjectCardOptions{
		ContentID:   issue.GetID(),
		ContentType: "Issue",
	}

	log.Printf("Creating card for issue #%d '%s', in column '%s' #%d; %#v", issue.GetNumber(), issue.GetTitle(), column.GetName(), column.GetID(), cardOpts)
	card, _, err := service.CreateProjectCard(repo.Context, column.GetID(), &cardOpts)

	if err != nil {
		return nil, err
	}

	// log.Printf("Card created: %d '%s'", card.GetID(), card.GetNote())

	return card, nil
}

// FetchMappedProjectColumns produces a string-map of the named columns in a project.
func (repo *WorkflowRepository) FetchMappedProjectColumns(project *github.Project) (map[string](*github.ProjectColumn), error) {
	var columnsFoundMap map[string](*github.ProjectColumn)
	columnsFound, err := repo.fetchProjectColumns(repo.Client.getProjectsService(), project)

	if err != nil {
		return nil, err
	}

	columnsFoundMap = make(map[string](*github.ProjectColumn))
	for _, col := range columnsFound {
		columnsFoundMap[col.GetName()] = col
	}

	return columnsFoundMap, nil
}

// ColumnsPresent indicates whether all the named columns are present in a project.
func (repo *WorkflowRepository) ColumnsPresent(project *github.Project, columns []string) (bool, error) {
	foundColumns, err := repo.fetchProjectColumns(repo.Client.getProjectsService(), project)
	if err != nil {
		return false, err
	}

	countMissing := len(columns) // presumably a positive (non-zero) number.

	for _, col := range foundColumns {
		for _, name := range columns {
			if name == col.GetName() {
				countMissing--
			}
		}
	}

	return (countMissing < 1), nil
}

// CheckTracks checks if a track tag exists in the slice the user passed down
func CheckTracks(tracks []string, inputTrack string) bool {
	for _, track := range tracks {
		if track == inputTrack {
			return true
		}
	}
	return false
}

// CardExists checks if a card has been created already in a different track
func CardExists(cards []string, inputCard string) bool {
	fmt.Println("in CardExists")
	for _, card := range cards {
		if card == inputCard {
			return true
		}
	}
	return false
}
