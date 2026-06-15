package reminder

import (
	"sync"
	"time"

	"pkgs/db/models"
	"pkgs/db/storage"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type FireHandler func(job *models.ReminderJob)

type Scheduler struct {
	mu      sync.Mutex
	db      *gorm.DB
	onFire  FireHandler
	timers  map[string]*time.Timer
	cron    *cron.Cron
	started bool
}

var defaultScheduler = &Scheduler{
	timers: make(map[string]*time.Timer),
}

func Default() *Scheduler {
	return defaultScheduler
}

func Init(db *gorm.DB, onFire FireHandler) error {
	if db == nil {
		return nil
	}
	s := Default()
	s.mu.Lock()
	s.db = db
	s.onFire = onFire
	if !s.started {
		s.cron = cron.New(cron.WithParser(cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)))
		s.cron.Start()
		s.started = true
	}
	s.mu.Unlock()
	return s.Reload()
}

func (s *Scheduler) Reload() error {
	jobs, err := storage.ListPendingReminderJobs(s.db)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, timer := range s.timers {
		timer.Stop()
		delete(s.timers, id)
	}
	for i := range jobs {
		job := jobs[i]
		s.scheduleLocked(&job)
	}
	return nil
}

func (s *Scheduler) Schedule(job *models.ReminderJob) error {
	if job == nil || job.ID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timers[job.ID] != nil {
		s.timers[job.ID].Stop()
		delete(s.timers, job.ID)
	}
	s.scheduleLocked(job)
	return nil
}

func (s *Scheduler) Cancel(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if timer, ok := s.timers[jobID]; ok {
		timer.Stop()
		delete(s.timers, jobID)
	}
}

func (s *Scheduler) scheduleLocked(job *models.ReminderJob) {
	if s.onFire == nil || job == nil || job.Status != models.ReminderStatusPending {
		return
	}
	var runAt time.Time
	switch job.ScheduleKind {
	case ScheduleCron:
		if job.NextRunAt != nil {
			runAt = *job.NextRunAt
		} else if job.CronExpr != "" {
			next, err := NextCronRun(job.CronExpr, time.Now())
			if err != nil {
				return
			}
			runAt = next
		}
	case ScheduleRelative:
		if job.RunAt != nil {
			runAt = *job.RunAt
		}
	default:
		return
	}
	if runAt.IsZero() {
		return
	}
	delay := time.Until(runAt)
	if delay < 0 {
		delay = 0
	}
	jobID := job.ID
	s.timers[jobID] = time.AfterFunc(delay, func() {
		s.handleFire(jobID)
	})
}

func (s *Scheduler) handleFire(jobID string) {
	s.mu.Lock()
	db := s.db
	onFire := s.onFire
	delete(s.timers, jobID)
	s.mu.Unlock()
	if db == nil || onFire == nil {
		return
	}
	job, err := storage.GetReminderJobByID(db, jobID)
	if err != nil || job == nil || job.Status != models.ReminderStatusPending {
		return
	}
	onFire(job)
	if job.ScheduleKind == ScheduleCron {
		next, err := NextCronRun(job.CronExpr, time.Now())
		if err != nil {
			_ = storage.CancelReminderJob(db, job.ID)
			return
		}
		_ = storage.UpdateReminderJobFields(db, job.ID, map[string]interface{}{
			"next_run_at": next,
		})
		job.NextRunAt = &next
		s.Schedule(job)
		return
	}
	_ = storage.UpdateReminderJobFields(db, job.ID, map[string]interface{}{
		"status": models.ReminderStatusCompleted,
	})
}

func CreateJob(db *gorm.DB, taskID uint, sessionID, name, content, timeSpec string) (*models.ReminderJob, error) {
	s := Default()
	s.mu.Lock()
	if s.db == nil {
		s.db = db
	}
	s.mu.Unlock()

	kind, runAt, cronExpr, err := ParseTimeSpec(timeSpec)
	if err != nil {
		return nil, err
	}
	job := &models.ReminderJob{
		ID:           newJobID(),
		TaskID:       taskID,
		SessionID:    sessionID,
		Name:         name,
		Content:      content,
		TimeSpec:     timeSpec,
		ScheduleKind: kind,
		RunAt:        runAt,
		CronExpr:     cronExpr,
		NextRunAt:    runAt,
		Status:       models.ReminderStatusPending,
	}
	if err := storage.CreateReminderJob(db, job); err != nil {
		return nil, err
	}
	if err := Default().Schedule(job); err != nil {
		return nil, err
	}
	return job, nil
}

func RemoveJob(db *gorm.DB, jobID string) error {
	if err := storage.CancelReminderJob(db, jobID); err != nil {
		return err
	}
	Default().Cancel(jobID)
	return nil
}

var jobCounter int64

func newJobID() string {
	jobCounter++
	return fmtJobID(time.Now().UnixNano(), jobCounter)
}

func fmtJobID(nano int64, seq int64) string {
	return "remind-" + itoa(nano) + "-" + itoa(seq)
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	buf := make([]byte, 0, 20)
	for v > 0 {
		buf = append([]byte{byte('0' + v%10)}, buf...)
		v /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
