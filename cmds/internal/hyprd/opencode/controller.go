// Package opencode coordinates OpenCode managed-service refreshes and Kitty TUI recycling.
package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	ModeQueue   Mode = "queue"
	ModeNow     Mode = "now"
	ModeRecycle Mode = "recycle"

	queueCeiling  = 30 * time.Minute
	quietPoll     = 2 * time.Second
	healthCeiling = 30 * time.Second
	maxDetails    = 80
)

type Mode string

type Status struct {
	ID        string    `json:"id,omitempty"`
	Mode      Mode      `json:"mode,omitempty"`
	State     string    `json:"state"`
	Phase     string    `json:"phase,omitempty"`
	Message   string    `json:"message,omitempty"`
	Details   []string  `json:"details,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s Status) Terminal() bool {
	switch s.State {
	case "done", "error", "aborted", "canceled":
		return true
	default:
		return false
	}
}

type job struct {
	status Status
	notify bool
	cancel bool
	wake   chan struct{}
}

type result struct {
	state   string
	message string
	details []string
}

type Controller struct {
	mu      sync.Mutex
	ready   chan struct{}
	current *job
	pending *job
	recent  map[string]Status
	order   []string
	next    atomic.Uint64
	api     *apiClient
	notify  func(title, body string)
}

func New(notify func(title, body string)) *Controller {
	controller := &Controller{
		ready:  make(chan struct{}, 1),
		recent: make(map[string]Status),
		api:    newAPIClient(),
		notify: notify,
	}
	go controller.worker()
	return controller
}

func (c *Controller) Handle(args string) string {
	fields := strings.Fields(args)
	if len(fields) == 0 {
		return "error: usage: opencode {queue|now|recycle|status|cancel}"
	}
	switch fields[0] {
	case string(ModeQueue), string(ModeNow), string(ModeRecycle):
		if len(fields) != 1 {
			return "error: usage: opencode " + fields[0]
		}
		return c.enqueue(Mode(fields[0]))
	case "status":
		if len(fields) > 2 {
			return "error: usage: opencode status [job-id]"
		}
		id := ""
		if len(fields) == 2 {
			id = fields[1]
		}
		return c.status(id)
	case "cancel":
		if len(fields) > 2 {
			return "error: usage: opencode cancel [job-id]"
		}
		id := ""
		if len(fields) == 2 {
			id = fields[1]
		}
		return c.cancelJob(id)
	default:
		return "error: unknown opencode command: " + fields[0]
	}
}

func (c *Controller) enqueue(mode Mode) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if mode == ModeNow {
		if existing := c.fullJobLocked(); existing != nil {
			if existing.status.Mode == ModeQueue && (existing.status.State == "queued" || existing.status.State == "waiting") {
				existing.status.Mode = ModeNow
				existing.status.State = "running"
				existing.status.Phase = "promoted"
				existing.status.Message = "refresh promoted; gates bypassed"
				existing.status.UpdatedAt = time.Now()
				wake(existing.wake)
				return "job " + existing.status.ID
			}
			return "already " + existing.status.ID
		}
		c.dropPendingRecycleLocked("superseded by immediate refresh")
	}

	if mode == ModeQueue {
		if existing := c.fullJobLocked(); existing != nil {
			existing.notify = true
			return "already " + existing.status.ID
		}
		c.dropPendingRecycleLocked("superseded by queued refresh")
	}

	if mode == ModeRecycle {
		if existing := c.fullJobLocked(); existing != nil {
			return "already " + existing.status.ID
		}
		if existing := c.recycleJobLocked(); existing != nil {
			return "already " + existing.status.ID
		}
	}

	if c.pending != nil {
		return "error: refresh queue is full"
	}
	now := time.Now()
	id := "oc" + strconv.FormatUint(c.next.Add(1), 36)
	c.pending = &job{
		status: Status{ID: id, Mode: mode, State: "queued", Phase: "queued", Message: "waiting for worker", CreatedAt: now, UpdatedAt: now},
		notify: mode == ModeQueue,
		wake:   make(chan struct{}, 1),
	}
	wake(c.ready)
	return "job " + id
}

func (c *Controller) status(id string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	var status Status
	switch {
	case c.current != nil && (id == "" || c.current.status.ID == id):
		status = c.current.status
	case c.pending != nil && (id == "" || c.pending.status.ID == id):
		status = c.pending.status
	case id != "":
		var ok bool
		status, ok = c.recent[id]
		if !ok {
			return "error: unknown opencode job: " + id
		}
	default:
		status = Status{State: "idle"}
	}
	data, err := json.Marshal(status)
	if err != nil {
		return "error: encode opencode status: " + err.Error()
	}
	return string(data)
}

func (c *Controller) cancelJob(id string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	target := c.pending
	if id != "" && (target == nil || target.status.ID != id) {
		target = nil
	}
	if target == nil && c.current != nil && c.current.status.State == "waiting" && (id == "" || c.current.status.ID == id) {
		target = c.current
	}
	if target == nil {
		return "error: no matching waiting or queued job"
	}

	target.cancel = true
	target.status.State = "canceled"
	target.status.Phase = "canceled"
	target.status.Message = "canceled before refresh"
	target.status.UpdatedAt = time.Now()
	wake(target.wake)
	if target == c.pending {
		c.pending = nil
		c.rememberLocked(target.status)
	}
	return "canceled " + target.status.ID
}

func (c *Controller) worker() {
	for range c.ready {
		for {
			job := c.take()
			if job == nil {
				break
			}
			outcome := c.run(job)
			notify, status := c.complete(job, outcome)
			if notify && status.State != "canceled" && c.notify != nil {
				title := "OpenCode refresh " + status.State
				c.notify(title, notificationBody(status))
			}
		}
	}
}

func (c *Controller) take() *job {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current != nil || c.pending == nil {
		return nil
	}
	c.current = c.pending
	c.pending = nil
	c.current.status.State = "running"
	c.current.status.Phase = "starting"
	c.current.status.Message = "starting refresh"
	c.current.status.UpdatedAt = time.Now()
	return c.current
}

func (c *Controller) run(job *job) result {
	mode := c.mode(job)
	if mode == ModeRecycle {
		c.update(job, "running", "health", "checking OpenCode service health")
		if err := c.api.health(context.Background()); err != nil {
			return result{state: "error", message: "service is unavailable: " + err.Error()}
		}
	}

	c.update(job, "running", "snapshot", "capturing OpenCode TUI panes")
	snapshot, details, err := takeSnapshot(job.status.ID, mode)
	if err != nil {
		return result{state: "error", message: "snapshot failed: " + err.Error()}
	}

	if mode == ModeRecycle {
		block, skip := paneGates(snapshot)
		maps.Copy(skip, block)
		c.update(job, "running", "recycle", fmt.Sprintf("recycling %d TUI panes", len(snapshot.Panes)))
		details = append(details, recycle(snapshot, skip, false)...)
		return result{state: "done", message: recycleSummary(snapshot), details: details}
	}

	skip := make(map[paneKey]string)
	if c.mode(job) == ModeQueue {
		var gateResult result
		snapshot, skip, gateResult = c.waitForQuiet(job, snapshot)
		if gateResult.state != "" {
			gateResult.details = append(details, gateResult.details...)
			return gateResult
		}
		details = append(details, gateResult.details...)
	}

	c.update(job, "running", "snapshot", "recapturing OpenCode TUI panes")
	fresh, snapDetails, err := takeSnapshot(job.status.ID, c.mode(job))
	if err != nil {
		return result{state: "error", message: "snapshot failed: " + err.Error(), details: details}
	}
	snapshot = fresh
	details = append(details, snapDetails...)

	if !c.beginStop(job) {
		return result{state: "canceled", message: "queued refresh canceled", details: details}
	}
	active, activeErr := c.api.active(context.Background())
	c.update(job, "running", "close", fmt.Sprintf("closing %d TUI panes", len(snapshot.Panes)))
	closed, closeDetails := closeTUIs(snapshot, skip, c.mode(job) == ModeNow)
	details = append(details, closeDetails...)

	c.update(job, "running", "stop", "stopping the OpenCode service")
	if err := stopService(context.Background()); err != nil {
		details = append(details, relaunch(closed)...)
		return result{state: "error", message: "stop failed: " + err.Error(), details: details}
	}
	message := "OpenCode service refreshed"
	switch {
	case len(active) > 0:
		message += fmt.Sprintf("; interrupted %d active sessions, which the new service resumes", len(active))
	case activeErr != nil && !errors.Is(activeErr, errAbsent):
		message += "; active sessions unknown (" + activeErr.Error() + ")"
	}

	c.update(job, "running", "recycle", fmt.Sprintf("relaunching %d TUI panes", len(closed)))
	details = append(details, relaunch(closed)...)
	if len(closed) == 0 {
		return result{state: "done", message: message + "; service stopped until the next c starts it", details: details}
	}

	c.update(job, "running", "health", "waiting for the TUI-started OpenCode service")
	if err := c.api.waitHealthy(context.Background(), healthCeiling); err != nil {
		return result{state: "error", message: "service health failed: " + err.Error(), details: details}
	}
	return result{state: "done", message: message + "; " + recycleSummary(snapshot), details: details}
}

func (c *Controller) waitForQuiet(job *job, snapshot Snapshot) (Snapshot, map[paneKey]string, result) {
	deadline := time.Now().Add(queueCeiling)
	quiet := 0
	lastReason := "quiet window was incomplete"
	for {
		if c.canceled(job) {
			return snapshot, nil, result{state: "canceled", message: "queued refresh canceled"}
		}
		if c.mode(job) == ModeNow {
			return snapshot, map[paneKey]string{}, result{}
		}
		if !time.Now().Before(deadline) {
			if quiet < 3 {
				return snapshot, nil, result{state: "aborted", message: "queue ceiling reached: " + lastReason}
			}
			block, skip := paneGates(snapshot)
			for key, why := range block {
				skip[key] = why + " after queue ceiling"
			}
			return snapshot, skip, result{}
		}

		pollContext, cancel := context.WithDeadline(context.Background(), deadline)
		active, err := c.api.active(pollContext)
		cancel()
		busy, reason := len(active) > 0, strings.Join(active, ", ")+" running"
		switch {
		case err == nil:
		case errors.Is(err, errAbsent):
			c.update(job, "running", "gate", "service down; skipping quiet wait")
			_, skip := paneGates(snapshot)
			return snapshot, skip, result{details: []string{"service down; skipped quiet wait: " + err.Error()}}
		case errors.Is(err, errUnavailable):
			busy, reason = true, err.Error()
		default:
			return snapshot, nil, result{state: "error", message: "cannot observe OpenCode sessions: " + err.Error()}
		}
		if busy {
			quiet = 0
			lastReason = reason
		} else {
			quiet++
			lastReason = fmt.Sprintf("only %d consecutive quiet polls completed", quiet)
		}
		block, skip := paneGates(snapshot)
		message := fmt.Sprintf("quiet %d/3", quiet)
		if busy {
			message = "busy: " + reason
		}
		if len(block) > 0 {
			message += fmt.Sprintf("; %d focused or unknown panes deferred", len(block))
		}
		c.update(job, "waiting", "gate", message)

		if quiet >= 3 && len(block) == 0 {
			return snapshot, skip, result{}
		}

		wait := min(quietPoll, time.Until(deadline))
		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
		case <-job.wake:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		}

		fresh, _, snapErr := takeSnapshot(job.status.ID, ModeQueue)
		if snapErr != nil {
			continue
		}
		snapshot = fresh
	}
}

func (c *Controller) complete(job *job, outcome result) (bool, Status) {
	c.mu.Lock()
	defer c.mu.Unlock()
	job.status.State = outcome.state
	job.status.Phase = outcome.state
	job.status.Message = limit(outcome.message, 2048)
	job.status.Details = limitedDetails(outcome.details)
	job.status.UpdatedAt = time.Now()
	if c.current == job {
		c.current = nil
	}
	c.rememberLocked(job.status)
	return job.notify, job.status
}

func (c *Controller) update(job *job, state, phase, message string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	job.status.State = state
	job.status.Phase = phase
	job.status.Message = limit(message, 2048)
	job.status.UpdatedAt = time.Now()
}

func (c *Controller) RebuildBlocked() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	job := c.current
	if job == nil {
		job = c.pending
	}
	if job == nil {
		return ""
	}
	return "opencode refresh job " + job.status.ID + " in flight"
}

func (c *Controller) beginStop(job *job) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if job.cancel {
		return false
	}
	job.status.State = "running"
	job.status.Phase = "stop"
	job.status.Message = "preparing to stop the OpenCode service"
	job.status.UpdatedAt = time.Now()
	return true
}

func (c *Controller) mode(job *job) Mode {
	c.mu.Lock()
	defer c.mu.Unlock()
	return job.status.Mode
}

func (c *Controller) canceled(job *job) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return job.cancel
}

func (c *Controller) fullJobLocked() *job {
	if c.current != nil && c.current.status.Mode != ModeRecycle {
		return c.current
	}
	if c.pending != nil && c.pending.status.Mode != ModeRecycle {
		return c.pending
	}
	return nil
}

func (c *Controller) recycleJobLocked() *job {
	if c.current != nil && c.current.status.Mode == ModeRecycle {
		return c.current
	}
	if c.pending != nil && c.pending.status.Mode == ModeRecycle {
		return c.pending
	}
	return nil
}

func (c *Controller) dropPendingRecycleLocked(message string) {
	if c.pending == nil || c.pending.status.Mode != ModeRecycle {
		return
	}
	c.pending.status.State = "canceled"
	c.pending.status.Phase = "canceled"
	c.pending.status.Message = message
	c.pending.status.UpdatedAt = time.Now()
	c.rememberLocked(c.pending.status)
	c.pending = nil
}

func (c *Controller) rememberLocked(status Status) {
	c.recent[status.ID] = status
	c.order = append(c.order, status.ID)
	for len(c.order) > 16 {
		delete(c.recent, c.order[0])
		c.order = c.order[1:]
	}
}

func wake(channel chan struct{}) {
	select {
	case channel <- struct{}{}:
	default:
	}
}

func recycleSummary(snapshot Snapshot) string {
	if len(snapshot.Panes) == 0 {
		return "no TUI panes found"
	}
	return fmt.Sprintf("processed %d TUI panes", len(snapshot.Panes))
}

func notificationBody(status Status) string {
	body := status.Message
	if len(status.Details) > 0 {
		body += "; " + strings.Join(status.Details[:min(3, len(status.Details))], "; ")
	}
	return limit(body, 512)
}

func limitedDetails(details []string) []string {
	if len(details) > maxDetails {
		omitted := len(details) - maxDetails + 1
		details = append(details[:maxDetails-1], fmt.Sprintf("%d more pane results omitted", omitted))
	}
	for index := range details {
		details[index] = limit(details[index], 512)
	}
	return details
}

func limit(value string, maximum int) string {
	runes := []rune(value)
	if len(runes) <= maximum {
		return value
	}
	return string(runes[:maximum])
}
