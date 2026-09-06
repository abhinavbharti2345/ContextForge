package roo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/entireio/external-agents/agents/entire-agent-roo/internal/protocol"
)

type HookEmitter interface {
	Emit(hookName string, payload RooHookPayload) error
}

type DefaultHookEmitter struct {
	EntireCommand string
	RepoRoot      string
}

func (e *DefaultHookEmitter) Emit(hookName string, payload RooHookPayload) error {
	cmdName := e.EntireCommand
	if cmdName == "" {
		cmdName = "entire"
	}

	repoRoot := e.RepoRoot
	if repoRoot == "" {
		repoRoot = protocol.RepoRoot()
	}

	// Just invoke it for now, RooHookPayload marshal logic isn't here but assuming it's in protocol or json works
	data, _ := json.Marshal(payload)

	cmd := exec.Command(cmdName, "hooks", "roo", hookName)
	if repoRoot != "" {
		cmd.Dir = repoRoot
	}
	cmd.Stdin = strings.NewReader(string(data) + "\n")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

type TaskWatcherState struct {
	TaskID               string
	SessionStarted       bool
	LastEmittedTurnIndex int
	LastEmittedPrompt    string
	InFlight             bool
	SessionEnded         bool
	LastSettledSignature string
	LastUpdated          time.Time
}

type Watcher struct {
	tasksDir         string
	repoRoot         string
	pollInterval     time.Duration
	debounceDuration time.Duration
	emitter          HookEmitter
	states           map[string]*TaskWatcherState
	mu               sync.Mutex
}

type WatcherOptions struct {
	TasksDir         string
	RepoRoot         string
	PollInterval     time.Duration
	DebounceDuration time.Duration
	Emitter          HookEmitter
}

func NewWatcher(opts WatcherOptions) *Watcher {
	tasksDir := opts.TasksDir
	if tasksDir == "" {
		tasksDir = GetGlobalStorageTasksDir()
	}

	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		repoRoot = protocol.RepoRoot()
	}

	pollInterval := opts.PollInterval
	if pollInterval <= 0 {
		pollInterval = 250 * time.Millisecond
	}

	debounce := opts.DebounceDuration
	if debounce <= 0 {
		debounce = 500 * time.Millisecond
	}

	emitter := opts.Emitter
	if emitter == nil {
		emitter = &DefaultHookEmitter{
			EntireCommand: "entire",
			RepoRoot:      repoRoot,
		}
	}

	w := &Watcher{
		tasksDir:         tasksDir,
		repoRoot:         repoRoot,
		pollInterval:     pollInterval,
		debounceDuration: debounce,
		emitter:          emitter,
		states:           make(map[string]*TaskWatcherState),
	}

	w.BootstrapExistingTasks()
	return w
}

func readSessionData(taskFolder string) ([]byte, error) {
	jsonlPath := filepath.Join(taskFolder, "transcript.jsonl")
	uiPath := filepath.Join(taskFolder, "ui_messages.json")

	if data, err := os.ReadFile(jsonlPath); err == nil {
		return data, nil
	}
	if data, err := os.ReadFile(uiPath); err == nil {
		return data, nil
	}
	return nil, os.ErrNotExist
}

func (w *Watcher) BootstrapExistingTasks() {
	if w.tasksDir == "" {
		return
	}
	entries, err := os.ReadDir(w.tasksDir)
	if err != nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		taskID := entry.Name()
		taskFolder := filepath.Join(w.tasksDir, taskID)

		data, err := readSessionData(taskFolder)
		if err != nil {
			continue
		}

		session, err := LoadSession(data)
		if err != nil {
			continue
		}

		lastPromptIdx, _, _ := DetectNewPrompt(session, -1)
		sig := calculateContentSignature(session)
		isTurnDone := IsTurnCompleted(session)
		isTaskDone := IsTaskCompleted(session)

		settledSig := sig
		inFlight := false
		if !isTurnDone && !isTaskDone {
			// Task is actively running mid-turn at watcher boot time
			inFlight = true
			settledSig = ""
		}

		w.states[taskID] = &TaskWatcherState{
			TaskID:               taskID,
			SessionStarted:       true,
			LastEmittedTurnIndex: lastPromptIdx,
			InFlight:             inFlight,
			SessionEnded:         isTaskDone,
			LastSettledSignature: settledSig,
			LastUpdated:          time.Now(),
		}
	}
}

func (w *Watcher) ProcessTask(taskID string, session NormalizedSession) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	state, exists := w.states[taskID]
	if !exists {
		state = &TaskWatcherState{
			TaskID:               taskID,
			LastEmittedTurnIndex: -1,
		}
		w.states[taskID] = state
	}

	if len(session.Events) == 0 {
		return nil
	}

	sessionRef := transcriptPath(taskID)

	// 1. SessionStart check
	if !state.SessionStarted {
		payload := RooHookPayload{
			Event:      "SessionStart",
			HookEvent:  "SessionStart",
			SessionID:  taskID,
			TaskID:     taskID,
			SessionRef: sessionRef,
			WorkingDir: w.repoRoot,
		}
		_ = w.emitter.Emit("session-start", payload)
		state.SessionStarted = true
	}

	// 2. New Prompt / TurnStart detection
	newPromptIndex, newPrompt, hasNewPrompt := DetectNewPrompt(session, state.LastEmittedTurnIndex)
	if hasNewPrompt {
		state.LastEmittedTurnIndex = newPromptIndex
		state.LastEmittedPrompt = newPrompt
		state.InFlight = true
		state.SessionEnded = false

		payload := RooHookPayload{
			Event:      "UserPromptSubmit",
			HookEvent:  "UserPromptSubmit",
			SessionID:  taskID,
			TaskID:     taskID,
			SessionRef: sessionRef,
			Prompt:     newPrompt,
			UserPrompt: newPrompt,
			Message:    newPrompt,
			WorkingDir: w.repoRoot,
		}
		_ = w.emitter.Emit("turn-start", payload)
	}

	// 3. Turn completion check
	sig := calculateContentSignature(session)
	if state.InFlight && IsTurnCompleted(session) {
		if sig != state.LastSettledSignature {
			state.LastSettledSignature = sig
			state.InFlight = false

			payload := RooHookPayload{
				Event:      "Stop",
				HookEvent:  "Stop",
				SessionID:  taskID,
				TaskID:     taskID,
				SessionRef: sessionRef,
				WorkingDir: w.repoRoot,
			}
			_ = w.emitter.Emit("turn-end", payload)
		}
	}

	// 4. Task completion / SessionEnd check
	if !state.SessionEnded && IsTaskCompleted(session) {
		state.SessionEnded = true
		payload := RooHookPayload{
			Event:      "SessionEnd",
			HookEvent:  "SessionEnd",
			SessionID:  taskID,
			TaskID:     taskID,
			SessionRef: sessionRef,
			WorkingDir: w.repoRoot,
		}
		_ = w.emitter.Emit("session-end", payload)
	}

	return nil
}

func (w *Watcher) ScanOnce() {
	if w.tasksDir == "" {
		return
	}

	entries, err := os.ReadDir(w.tasksDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		taskID := entry.Name()
		taskFolder := filepath.Join(w.tasksDir, taskID)

		data, err := readSessionData(taskFolder)
		if err != nil {
			continue
		}

		session, err := LoadSession(data)
		if err != nil {
			continue
		}

		_ = w.ProcessTask(taskID, session)
	}
}

func (w *Watcher) Watch(ctx context.Context) error {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			w.ScanOnce()
		}
	}
}

func DetectNewPrompt(session NormalizedSession, lastTurnIndex int) (int, string, bool) {
	events := session.Events
	for i := len(events) - 1; i > lastTurnIndex; i-- {
		msg := events[i]
		if msg.Type == EventUserPrompt {
			text := strings.TrimSpace(msg.Text)
			if text != "" {
				return i, text, true
			}
		}
	}
	return -1, "", false
}

func IsTurnCompleted(session NormalizedSession) bool {
	if len(session.Events) == 0 {
		return false
	}
	if session.Partial {
		return false
	}

	lastEvent := session.Events[len(session.Events)-1]
	
	// Check if the event signifies completion explicitly
	if lastEvent.IsTurnComplete || lastEvent.IsTaskComplete {
		return true
	}
	
	return false
}

func IsTaskCompleted(session NormalizedSession) bool {
	if len(session.Events) == 0 {
		return false
	}
	if session.Partial {
		return false
	}
	lastEvent := session.Events[len(session.Events)-1]
	return lastEvent.IsTaskComplete
}

func calculateContentSignature(session NormalizedSession) string {
	hasher := sha256.New()
	for _, m := range session.Events {
		hasher.Write(m.Raw)
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func RunWatcher(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(stderr)

	tasksDir := fs.String("tasks-dir", "", "path to Roo Code tasks directory")
	repoRoot := fs.String("repo-root", "", "path to target git repository root")
	pollMs := fs.Int("poll-interval", 250, "poll interval in milliseconds")
	debounceMs := fs.Int("debounce", 500, "debounce interval in milliseconds")
	entireCmd := fs.String("entire-cmd", "entire", "entire CLI executable name or path")

	if err := fs.Parse(args); err != nil {
		return err
	}

	dir := *tasksDir
	if dir == "" {
		dir = GetGlobalStorageTasksDir()
	}
	if dir == "" {
		return fmt.Errorf("could not resolve Roo Code tasks storage directory; please pass --tasks-dir")
	}

	root := *repoRoot
	if root == "" {
		root = protocol.RepoRoot()
	}

	_, _ = fmt.Fprintf(stdout, "Starting entire-agent-roo watcher on tasks dir: %s\n", dir)
	_, _ = fmt.Fprintf(stdout, "Target repository root: %s\n", root)
	_, _ = fmt.Fprintf(stdout, "Poll interval: %dms | Debounce: %dms\n", *pollMs, *debounceMs)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		_, _ = fmt.Fprintln(stdout, "\nShutting down watcher...")
		cancel()
	}()

	watcher := NewWatcher(WatcherOptions{
		TasksDir:         dir,
		RepoRoot:         root,
		PollInterval:     time.Duration(*pollMs) * time.Millisecond,
		DebounceDuration: time.Duration(*debounceMs) * time.Millisecond,
		Emitter: &DefaultHookEmitter{
			EntireCommand: *entireCmd,
			RepoRoot:      root,
		},
	})

	return watcher.Watch(ctx)
}
