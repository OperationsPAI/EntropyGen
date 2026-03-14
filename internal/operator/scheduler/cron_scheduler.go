package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"github.com/entropyGen/entropyGen/internal/common/models"
	"github.com/entropyGen/entropyGen/internal/common/redisclient"
)

// CronScheduler manages cron jobs that write trigger events to Redis Streams.
type CronScheduler struct {
	writer *redisclient.StreamWriter
	cron   *cron.Cron
	mu     sync.Mutex
	jobs   map[string]jobEntry // keyed by agent name
}

type jobEntry struct {
	entryID  cron.EntryID
	schedule string
}

// New creates a CronScheduler that publishes trigger events via the given StreamWriter.
func New(writer *redisclient.StreamWriter) *CronScheduler {
	return &CronScheduler{
		writer: writer,
		cron:   cron.New(),
		jobs:   make(map[string]jobEntry),
	}
}

// Sync adds or updates a cron job for the given agent.
// If the schedule hasn't changed, the existing job is kept.
func (s *CronScheduler) Sync(agentName, schedule, prompt string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.jobs[agentName]; ok {
		if existing.schedule == schedule {
			return // schedule unchanged
		}
		s.cron.Remove(existing.entryID)
		delete(s.jobs, agentName)
	}

	stream := fmt.Sprintf("events:agent-%s", agentName)
	entryID, err := s.cron.AddFunc(schedule, func() {
		s.fire(stream, agentName, prompt)
	})
	if err != nil {
		// Invalid schedule expression — skip silently.
		// The reconciler will retry on next reconcile.
		return
	}

	s.jobs[agentName] = jobEntry{entryID: entryID, schedule: schedule}
}

// Remove deletes the cron job for the given agent.
func (s *CronScheduler) Remove(agentName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.jobs[agentName]; ok {
		s.cron.Remove(existing.entryID)
		delete(s.jobs, agentName)
	}
}

// Start begins running scheduled jobs.
func (s *CronScheduler) Start() {
	s.cron.Start()
}

// Stop gracefully stops the scheduler and waits for running jobs to finish.
func (s *CronScheduler) Stop() {
	s.cron.Stop()
}

func (s *CronScheduler) fire(stream, agentName, prompt string) {
	payload, _ := json.Marshal(models.CronTriggerPayload{Prompt: prompt})

	event := &models.Event{
		EventID:   uuid.NewString(),
		EventType: models.EventTypeCronTrigger,
		Timestamp: time.Now().UTC(),
		AgentID:   agentName,
		Payload:   payload,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = s.writer.Write(ctx, stream, event, redisclient.MaxLenAgent)
}
