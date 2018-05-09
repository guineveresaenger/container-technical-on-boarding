package jobs

import (
	"time"

	"github.com/revel/cron"

	"github.com/revel/modules/jobs/app/jobs"
)

// Event of a job
type Event struct {
	SessionID     int    // The user session id
	Type          string // "start", "progress", "complete", and "error"
	Timestamp     int    // Unix timestamp (secs)
	Text          string // What the job progress is (if Type == "progress")
	Error         string // Source error (if Type == "error")
	ProjectNumber string // Number of the created project board
}

// NewEvent creates a new job event
func NewEvent(sid int, typ string, msg string) Event {
	return Event{sid, typ, int(time.Now().Unix()), msg, "", ""}
}

// NewProjectEvent contains projectID event
func NewProjectEvent(sid int, typ string, msg string, pnum string) Event {
	return Event{sid, typ, int(time.Now().Unix()), msg, "", pnum}
}

// NewError creates a new job error event
func NewError(sid int, msg string, err string) Event {
	return Event{sid, "error", int(time.Now().Unix()), msg, err, ""}
}

// StartJob is used start the revel job
func StartJob(job cron.Job) {
	jobs.Now(job)
}
