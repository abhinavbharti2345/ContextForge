package roo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	cmd := exec.Command(cmdName, "hooks", "roo", hookName)
	if e.RepoRoot != "" {
		cmd.Dir = e.RepoRoot
	}
	cmd.Stdin = strings.NewReader(string(data) + "\n")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

type TaskWatcherState struct {
	TaskID                string
	SessionStarted        bool
	LastEmittedTurnIndex  int
	LastEmittedPrompt     string
	InFlight              bool
	SessionEnded          bool
	LastSettledSignature  string
	LastUpdated           time.Time
}

type Watcher struct {
	tasksDir         string
	pollInterval     time.Duration
	debounceDuration time.Duration
	emitter          HookEmitter
	states           map[string]*TaskWatcherState
	mu               sync.Mutex
}

type WatcherOptions struct {
	TasksDir         string
	PollInterval     time.Duration
	DebounceDuration time.Duration
	Emitter          HookEmitter
}

func NewWatcher(opts WatcherOptions) *Watcher {
	tasksDir := opts.TasksDir
	if tasksDir == "" {
		tasksDir = GetGlobalStorageTasksDir()
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
			RepoRoot:      GetGlobalStorageTasksDir(),
		}
	}

	return &Watcher{
		tasksDir:         tasksDir,
		pollInterval:     pollInterval,
		debounceDuration: debounce,
		emitter:          emitter,
		states:           make(map[string]*TaskWatcherState),
	}
}

func (w *Watcher) ProcessTask(taskID string, uiMessages []ClineMessage, apiHistory []ApiMessage) error {
	w.mu.Lock()
	state, exists := w.states[taskID]
	if !exists {
		state = &TaskWatcherState{
			TaskID:               taskID,
			LastEmittedTurnIndex: -1,
		}
		w.states[taskID] = state
	}
	w.mu.Unlock()

	if len(uiMessages) == 0 {
		return nil
	}

	// 1. SessionStart check
	if !state.SessionStarted {
		payload := RooHookPayload{
			Event:     "SessionStart",
			HookEvent: "SessionStart",
			SessionID: taskID,
			TaskID:    taskID,
		}
		_ = w.emitter.Emit("session-start", payload)
		state.SessionStarted = true
	}

	// 2. New Prompt / TurnStart detection
	newPromptIndex, newPrompt, hasNewPrompt := DetectNewPrompt(uiMessages, state.LastEmittedTurnIndex)
	if hasNewPrompt {
		state.LastEmittedTurnIndex = newPromptIndex
		state.LastEmittedPrompt = newPrompt
		state.InFlight = true

		payload := RooHookPayload{
			Event:     "UserPromptSubmit",
			HookEvent: "UserPromptSubmit",
			SessionID: taskID,
			TaskID:    taskID,
			Prompt:    newPrompt,
			Message:   newPrompt,
		}
		_ = w.emitter.Emit("turn-start", payload)
	}

	// 3. Turn completion check
	sig := calculateContentSignature(uiMessages)
	if state.InFlight && IsTurnCompleted(uiMessages, apiHistory) {
		if sig != state.LastSettledSignature {
			state.LastSettledSignature = sig
			state.InFlight = false

			payload := RooHookPayload{
				Event:     "Stop",
				HookEvent: "Stop",
				SessionID: taskID,
				TaskID:    taskID,
			}
			_ = w.emitter.Emit("turn-end", payload)
		}
	}

	// 4. Task completion / SessionEnd check
	if !state.SessionEnded && IsTaskCompleted(uiMessages) {
		state.SessionEnded = true
		payload := RooHookPayload{
			Event:     "SessionEnd",
			HookEvent: "SessionEnd",
			SessionID: taskID,
			TaskID:    taskID,
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
		uiPath := filepath.Join(taskFolder, "ui_messages.json")
		apiPath := filepath.Join(taskFolder, "api_conversation_history.json")

		uiData, err := os.ReadFile(uiPath)
		if err != nil {
			continue
		}

		var uiMessages []ClineMessage
		if err := json.Unmarshal(uiData, &uiMessages); err != nil {
			continue
		}

		var apiHistory []ApiMessage
		if apiData, err := os.ReadFile(apiPath); err == nil {
			_ = json.Unmarshal(apiData, &apiHistory)
		}

		_ = w.ProcessTask(taskID, uiMessages, apiHistory)
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

func DetectNewPrompt(uiMessages []ClineMessage, lastTurnIndex int) (int, string, bool) {
	for i := len(uiMessages) - 1; i > lastTurnIndex; i-- {
		msg := uiMessages[i]
		if (msg.Type == "say" && (msg.Say == "task" || msg.Say == "user_feedback")) ||
			(msg.Type == "ask" && msg.Ask == "followup" && msg.Text != "") {
			text := strings.TrimSpace(msg.Text)
			if text != "" {
				return i, text, true
			}
		}
	}
	return -1, "", false
}

func IsTurnCompleted(uiMessages []ClineMessage, apiHistory []ApiMessage) bool {
	if len(uiMessages) == 0 {
		return false
	}

	lastUI := uiMessages[len(uiMessages)-1]

	// 1. Streaming message is NOT completed
	if lastUI.Partial {
		return false
	}

	// 2. In-flight tool or API execution is NOT completed
	if lastUI.Say == "api_req_started" || lastUI.Say == "tool" || lastUI.Say == "command" {
		return false
	}

	// 3. Waiting for user tool/command approval is paused, not completed
	if lastUI.Type == "ask" && (lastUI.Ask == "tool" || lastUI.Ask == "command") {
		return false
	}

	// 4. Completed states
	if lastUI.Say == "text" || lastUI.Say == "completion_result" || lastUI.Ask == "followup" {
		if len(apiHistory) > 0 {
			lastAPI := apiHistory[len(apiHistory)-1]
			if lastAPI.Role == "assistant" {
				return true
			}
		} else {
			return true
		}
	}

	return false
}

func IsTaskCompleted(uiMessages []ClineMessage) bool {
	if len(uiMessages) == 0 {
		return false
	}
	lastUI := uiMessages[len(uiMessages)-1]
	return lastUI.Say == "completion_result" && !lastUI.Partial
}

func calculateContentSignature(uiMessages []ClineMessage) string {
	hasher := sha256.New()
	for _, m := range uiMessages {
		hasher.Write([]byte(fmt.Sprintf("%d:%s:%s:%t:%s\n", m.Ts, m.Type, m.Say, m.Partial, m.Text)))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func RunWatcher(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(stderr)

	tasksDir := fs.String("tasks-dir", "", "path to Roo Code tasks directory")
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

	_, _ = fmt.Fprintf(stdout, "Starting entire-agent-roo watcher on: %s\n", dir)
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
		PollInterval:     time.Duration(*pollMs) * time.Millisecond,
		DebounceDuration: time.Duration(*debounceMs) * time.Millisecond,
		Emitter: &DefaultHookEmitter{
			EntireCommand: *entireCmd,
		},
	})

	return watcher.Watch(ctx)
}
