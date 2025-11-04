# Agent Decomposition Example

This document provides concrete examples for refactoring the Agent struct.

## Current State (agent.go - 1019 lines)

```go
// Current: God object with 16+ fields handling multiple concerns
type Agent struct {
    lock sync.RWMutex
    dry bool
    retryTarget *execution.DAGRunStatus
    dagStore execution.DAGStore
    dagRunStore execution.DAGRunStore
    registry execution.ServiceRegistry
    peerConfig config.Peer
    dagRunMgr runtime1.Manager
    scheduler *runtime.Scheduler
    graph *runtime.ExecutionGraph
    reporter *reporter
    socketServer *sock.Server
    logDir string
    logFile string
    dag *core.DAG
    rootDAGRun execution.DAGRunRef
    parentDAGRun execution.DAGRunRef
    dagRunID string
    dagRunAttemptID string
    finished atomic.Bool
    lastErr error
    isSubDAGRun atomic.Bool
    progressDisplay ProgressReporter
    stepRetry string
    tracer *telemetry.Tracer
}

// Current: Large methods with mixed concerns
func (a *Agent) Run(ctx context.Context) error { /* 200+ lines */ }
func (a *Agent) handleHTTP(w http.ResponseWriter, r *http.Request) { /* 150+ lines */ }
func (a *Agent) setupSocketServer() error { /* 100+ lines */ }
func (a *Agent) reporter() *reporter { /* 80+ lines */ }
```

## Proposed State (Multiple Focused Components)

### 1. Execution Agent Core (agent/agent.go - ~300 lines)

```go
package agent

import (
    "context"
    "fmt"
    "sync/atomic"
    
    "github.com/dagu-org/dagu/internal/runtime/agent/executor"
    "github.com/dagu-org/dagu/internal/runtime/agent/reporter"
    "github.com/dagu-org/dagu/internal/runtime/agent/socket"
    "github.com/dagu-org/dagu/internal/runtime/agent/progress"
)

// Agent orchestrates DAG execution
type Agent struct {
    executor  executor.Executor
    reporter  reporter.Reporter
    socket    socket.Handler
    progress  progress.Manager
    config    *Config
    
    // Execution state
    runID       string
    attemptID   string
    finished    atomic.Bool
    lastErr     error
}

// Config holds agent configuration
type Config struct {
    DAG             *core.DAG
    RunID           string
    AttemptID       string
    DryRun          bool
    RetryTarget     *execution.DAGRunStatus
    ParentRun       execution.DAGRunRef
    RootRun         execution.DAGRunRef
    StepRetry       string
    LogDir          string
    ShowProgress    bool
}

// New creates a new execution agent
func New(
    exec executor.Executor,
    rep reporter.Reporter,
    sock socket.Handler,
    prog progress.Manager,
    cfg *Config,
) *Agent {
    return &Agent{
        executor:  exec,
        reporter:  rep,
        socket:    sock,
        progress:  prog,
        config:    cfg,
        runID:     cfg.RunID,
        attemptID: cfg.AttemptID,
    }
}

// Run executes the DAG
func (a *Agent) Run(ctx context.Context) error {
    if a.finished.Load() {
        return ErrAlreadyFinished
    }
    
    // Report start
    if err := a.reporter.ReportStart(ctx, a.buildStartInfo()); err != nil {
        return fmt.Errorf("failed to report start: %w", err)
    }
    
    // Start socket server for control
    if err := a.socket.Start(ctx); err != nil {
        return fmt.Errorf("failed to start socket server: %w", err)
    }
    defer a.socket.Stop()
    
    // Start progress display
    if a.config.ShowProgress {
        if err := a.progress.Start(ctx); err != nil {
            log.Warn("failed to start progress display", "error", err)
        }
        defer a.progress.Stop()
    }
    
    // Execute DAG
    result, err := a.executeDAG(ctx)
    a.lastErr = err
    
    // Report completion
    if err := a.reporter.ReportComplete(ctx, result); err != nil {
        log.Error("failed to report completion", "error", err)
    }
    
    a.finished.Store(true)
    return err
}

// executeDAG performs the actual DAG execution
func (a *Agent) executeDAG(ctx context.Context) (*executor.Result, error) {
    // Prepare execution request
    req := &executor.Request{
        DAG:         a.config.DAG,
        DryRun:      a.config.DryRun,
        RetryTarget: a.config.RetryTarget,
        StepRetry:   a.config.StepRetry,
    }
    
    // Execute
    result, err := a.executor.Execute(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("execution failed: %w", err)
    }
    
    return result, nil
}

// Stop gracefully stops the agent
func (a *Agent) Stop(ctx context.Context) error {
    if a.finished.Load() {
        return nil
    }
    
    // Stop executor
    if err := a.executor.Stop(ctx); err != nil {
        return fmt.Errorf("failed to stop executor: %w", err)
    }
    
    a.finished.Store(true)
    return nil
}

// GetStatus returns the current execution status
func (a *Agent) GetStatus() *Status {
    return &Status{
        RunID:     a.runID,
        AttemptID: a.attemptID,
        Finished:  a.finished.Load(),
        Error:     a.lastErr,
        Executor:  a.executor.GetStatus(),
    }
}

// buildStartInfo creates start information for reporting
func (a *Agent) buildStartInfo() *reporter.StartInfo {
    return &reporter.StartInfo{
        RunID:     a.runID,
        AttemptID: a.attemptID,
        DAGName:   a.config.DAG.Name,
        ParentRun: a.config.ParentRun,
        RootRun:   a.config.RootRun,
        StartedAt: time.Now(),
    }
}
```

### 2. Executor (executor/executor.go - ~250 lines)

```go
package executor

import (
    "context"
    "fmt"
    "sync"
    
    "github.com/dagu-org/dagu/internal/core"
    "github.com/dagu-org/dagu/internal/runtime"
)

// Executor handles DAG execution logic
type Executor interface {
    Execute(ctx context.Context, req *Request) (*Result, error)
    Stop(ctx context.Context) error
    GetStatus() *Status
}

// Request contains execution parameters
type Request struct {
    DAG         *core.DAG
    DryRun      bool
    RetryTarget *execution.DAGRunStatus
    StepRetry   string
}

// Result contains execution results
type Result struct {
    Status      execution.Status
    StartedAt   time.Time
    FinishedAt  time.Time
    Steps       []*StepResult
    Error       error
}

// StepResult contains step execution result
type StepResult struct {
    Name       string
    Status     execution.Status
    StartedAt  time.Time
    FinishedAt time.Time
    Output     string
    Error      error
}

type executor struct {
    scheduler runtime.Scheduler
    graph     *runtime.ExecutionGraph
    reporter  Reporter
    config    *Config
    
    mu       sync.RWMutex
    running  atomic.Bool
    status   *Status
}

// Config holds executor configuration
type Config struct {
    MaxConcurrency int
    LogDir         string
    Timeout        time.Duration
}

// NewExecutor creates a new executor
func NewExecutor(
    scheduler runtime.Scheduler,
    reporter Reporter,
    cfg *Config,
) Executor {
    return &executor{
        scheduler: scheduler,
        reporter:  reporter,
        config:    cfg,
        status:    &Status{},
    }
}

// Execute executes the DAG
func (e *executor) Execute(ctx context.Context, req *Request) (*Result, error) {
    if !e.running.CompareAndSwap(false, true) {
        return nil, ErrAlreadyRunning
    }
    defer e.running.Store(false)
    
    // Build execution graph
    graph, err := runtime.NewExecutionGraph(req.DAG)
    if err != nil {
        return nil, fmt.Errorf("failed to build execution graph: %w", err)
    }
    e.graph = graph
    
    // Handle retry
    if req.RetryTarget != nil {
        if err := e.setupRetry(req.RetryTarget, req.StepRetry); err != nil {
            return nil, fmt.Errorf("failed to setup retry: %w", err)
        }
    }
    
    // Execute
    result := &Result{
        Status:    execution.StatusRunning,
        StartedAt: time.Now(),
    }
    
    if req.DryRun {
        result = e.executeDryRun(ctx, graph)
    } else {
        result = e.executeReal(ctx, graph)
    }
    
    result.FinishedAt = time.Now()
    return result, result.Error
}

// executeReal performs real execution
func (e *executor) executeReal(ctx context.Context, graph *runtime.ExecutionGraph) *Result {
    // Create scheduler context
    schedCtx := &runtime.SchedulerContext{
        Context: ctx,
        Graph:   graph,
        LogDir:  e.config.LogDir,
    }
    
    // Run scheduler
    if err := e.scheduler.Start(schedCtx); err != nil {
        return &Result{
            Status: execution.StatusError,
            Error:  err,
        }
    }
    
    // Wait for completion
    if err := e.scheduler.Wait(ctx); err != nil {
        return &Result{
            Status: execution.StatusError,
            Error:  err,
        }
    }
    
    // Build result
    return e.buildResult(graph)
}

// executeDryRun performs dry run
func (e *executor) executeDryRun(ctx context.Context, graph *runtime.ExecutionGraph) *Result {
    var steps []*StepResult
    
    for _, node := range graph.Nodes() {
        steps = append(steps, &StepResult{
            Name:       node.Step.Name,
            Status:     execution.StatusSuccess,
            StartedAt:  time.Now(),
            FinishedAt: time.Now(),
            Output:     "[DRY RUN] Would execute: " + node.Step.Command,
        })
    }
    
    return &Result{
        Status: execution.StatusSuccess,
        Steps:  steps,
    }
}

// Stop stops the execution
func (e *executor) Stop(ctx context.Context) error {
    if !e.running.Load() {
        return nil
    }
    
    if err := e.scheduler.Stop(ctx); err != nil {
        return fmt.Errorf("failed to stop scheduler: %w", err)
    }
    
    return nil
}

// GetStatus returns current status
func (e *executor) GetStatus() *Status {
    e.mu.RLock()
    defer e.mu.RUnlock()
    return e.status
}

// setupRetry prepares the executor for retry execution
func (e *executor) setupRetry(target *execution.DAGRunStatus, stepRetry string) error {
    if stepRetry != "" {
        // Retry specific step
        return e.graph.MarkStepForRetry(stepRetry)
    }
    
    // Retry failed steps
    for _, step := range target.Steps {
        if step.Status == execution.StatusError {
            if err := e.graph.MarkStepForRetry(step.Name); err != nil {
                return err
            }
        }
    }
    
    return nil
}

// buildResult builds execution result from graph
func (e *executor) buildResult(graph *runtime.ExecutionGraph) *Result {
    var steps []*StepResult
    status := execution.StatusSuccess
    
    for _, node := range graph.Nodes() {
        stepResult := &StepResult{
            Name:       node.Step.Name,
            Status:     node.Status,
            StartedAt:  node.StartedAt,
            FinishedAt: node.FinishedAt,
            Output:     node.Output,
            Error:      node.Error,
        }
        steps = append(steps, stepResult)
        
        if node.Status == execution.StatusError {
            status = execution.StatusError
        }
    }
    
    return &Result{
        Status: status,
        Steps:  steps,
    }
}

// Reporter interface for execution events
type Reporter interface {
    ReportStepStart(ctx context.Context, step string) error
    ReportStepComplete(ctx context.Context, step string, result *StepResult) error
}
```

### 3. Status Reporter (reporter/reporter.go - ~200 lines)

```go
package reporter

import (
    "context"
    "fmt"
    "sync"
    
    "github.com/dagu-org/dagu/internal/core/execution"
)

// Reporter handles status reporting and notifications
type Reporter interface {
    ReportStart(ctx context.Context, info *StartInfo) error
    ReportProgress(ctx context.Context, status *Status) error
    ReportComplete(ctx context.Context, result *Result) error
    Subscribe(listener Listener) error
    Unsubscribe(listener Listener) error
}

// StartInfo contains DAG run start information
type StartInfo struct {
    RunID     string
    AttemptID string
    DAGName   string
    ParentRun execution.DAGRunRef
    RootRun   execution.DAGRunRef
    StartedAt time.Time
}

// Status contains current execution status
type Status struct {
    RunID      string
    Status     execution.Status
    Steps      []*StepStatus
    UpdatedAt  time.Time
}

// StepStatus contains step execution status
type StepStatus struct {
    Name      string
    Status    execution.Status
    StartedAt time.Time
    Output    string
}

// Result contains execution result
type Result struct {
    RunID      string
    Status     execution.Status
    StartedAt  time.Time
    FinishedAt time.Time
    Error      error
}

// Listener receives status updates
type Listener interface {
    OnStart(info *StartInfo)
    OnProgress(status *Status)
    OnComplete(result *Result)
}

type reporter struct {
    store     execution.DAGRunStore
    notifier  Notifier
    listeners []Listener
    mu        sync.RWMutex
}

// Config holds reporter configuration
type Config struct {
    EnableNotifications bool
    NotificationConfig  *NotificationConfig
}

// NewReporter creates a new reporter
func NewReporter(store execution.DAGRunStore, notifier Notifier) Reporter {
    return &reporter{
        store:    store,
        notifier: notifier,
    }
}

// ReportStart reports DAG run start
func (r *reporter) ReportStart(ctx context.Context, info *StartInfo) error {
    // Create run record
    run := &execution.DAGRun{
        ID:        info.RunID,
        AttemptID: info.AttemptID,
        DAGName:   info.DAGName,
        Status:    execution.StatusRunning,
        StartedAt: info.StartedAt,
        ParentRun: info.ParentRun,
        RootRun:   info.RootRun,
    }
    
    if err := r.store.Create(ctx, run); err != nil {
        return fmt.Errorf("failed to create run record: %w", err)
    }
    
    // Notify listeners
    r.notifyStart(info)
    
    return nil
}

// ReportProgress reports execution progress
func (r *reporter) ReportProgress(ctx context.Context, status *Status) error {
    // Update run record
    if err := r.store.UpdateStatus(ctx, status.RunID, status.Status); err != nil {
        return fmt.Errorf("failed to update status: %w", err)
    }
    
    // Update step statuses
    for _, step := range status.Steps {
        if err := r.store.UpdateStepStatus(ctx, status.RunID, step.Name, step.Status); err != nil {
            log.Error("failed to update step status",
                "runID", status.RunID,
                "step", step.Name,
                "error", err)
        }
    }
    
    // Notify listeners
    r.notifyProgress(status)
    
    return nil
}

// ReportComplete reports execution completion
func (r *reporter) ReportComplete(ctx context.Context, result *Result) error {
    // Update run record
    run := &execution.DAGRun{
        ID:         result.RunID,
        Status:     result.Status,
        FinishedAt: result.FinishedAt,
        Error:      result.Error,
    }
    
    if err := r.store.Complete(ctx, run); err != nil {
        return fmt.Errorf("failed to complete run: %w", err)
    }
    
    // Send notifications
    if err := r.sendNotifications(ctx, result); err != nil {
        log.Error("failed to send notifications",
            "runID", result.RunID,
            "error", err)
    }
    
    // Notify listeners
    r.notifyComplete(result)
    
    return nil
}

// Subscribe adds a listener
func (r *reporter) Subscribe(listener Listener) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    r.listeners = append(r.listeners, listener)
    return nil
}

// Unsubscribe removes a listener
func (r *reporter) Unsubscribe(listener Listener) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    for i, l := range r.listeners {
        if l == listener {
            r.listeners = append(r.listeners[:i], r.listeners[i+1:]...)
            return nil
        }
    }
    
    return ErrListenerNotFound
}

// notifyStart notifies listeners of start
func (r *reporter) notifyStart(info *StartInfo) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    for _, listener := range r.listeners {
        go listener.OnStart(info)
    }
}

// notifyProgress notifies listeners of progress
func (r *reporter) notifyProgress(status *Status) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    for _, listener := range r.listeners {
        go listener.OnProgress(status)
    }
}

// notifyComplete notifies listeners of completion
func (r *reporter) notifyComplete(result *Result) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    for _, listener := range r.listeners {
        go listener.OnComplete(result)
    }
}

// sendNotifications sends email/webhook notifications
func (r *reporter) sendNotifications(ctx context.Context, result *Result) error {
    if r.notifier == nil {
        return nil
    }
    
    notification := &Notification{
        RunID:      result.RunID,
        Status:     result.Status,
        FinishedAt: result.FinishedAt,
        Error:      result.Error,
    }
    
    return r.notifier.Send(ctx, notification)
}

// Notifier interface for sending notifications
type Notifier interface {
    Send(ctx context.Context, notification *Notification) error
}

// Notification contains notification data
type Notification struct {
    RunID      string
    Status     execution.Status
    FinishedAt time.Time
    Error      error
}
```

### 4. Socket Handler (socket/handler.go - ~150 lines)

```go
package socket

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    
    "github.com/dagu-org/dagu/internal/common/sock"
)

// Handler handles Unix socket communication
type Handler interface {
    Start(ctx context.Context) error
    Stop() error
    RegisterCommand(name string, handler CommandHandler) error
}

// CommandHandler handles a specific command
type CommandHandler func(ctx context.Context, req *Request) (*Response, error)

// Request represents a socket request
type Request struct {
    Command string                 `json:"command"`
    Params  map[string]interface{} `json:"params"`
}

// Response represents a socket response
type Response struct {
    Success bool                   `json:"success"`
    Data    map[string]interface{} `json:"data,omitempty"`
    Error   string                 `json:"error,omitempty"`
}

type handler struct {
    server   *sock.Server
    handlers map[string]CommandHandler
    socketPath string
}

// NewHandler creates a new socket handler
func NewHandler(socketPath string) Handler {
    return &handler{
        socketPath: socketPath,
        handlers:   make(map[string]CommandHandler),
    }
}

// Start starts the socket server
func (h *handler) Start(ctx context.Context) error {
    mux := http.NewServeMux()
    mux.HandleFunc("/", h.handleRequest)
    
    server, err := sock.NewServer(h.socketPath, mux)
    if err != nil {
        return fmt.Errorf("failed to create socket server: %w", err)
    }
    
    h.server = server
    
    go func() {
        if err := server.Serve(ctx); err != nil {
            log.Error("socket server error", "error", err)
        }
    }()
    
    return nil
}

// Stop stops the socket server
func (h *handler) Stop() error {
    if h.server == nil {
        return nil
    }
    return h.server.Shutdown(context.Background())
}

// RegisterCommand registers a command handler
func (h *handler) RegisterCommand(name string, handler CommandHandler) error {
    if _, exists := h.handlers[name]; exists {
        return fmt.Errorf("command %s already registered", name)
    }
    h.handlers[name] = handler
    return nil
}

// handleRequest handles incoming socket requests
func (h *handler) handleRequest(w http.ResponseWriter, r *http.Request) {
    var req Request
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        h.writeError(w, "invalid request", http.StatusBadRequest)
        return
    }
    
    handler, ok := h.handlers[req.Command]
    if !ok {
        h.writeError(w, "unknown command", http.StatusNotFound)
        return
    }
    
    resp, err := handler(r.Context(), &req)
    if err != nil {
        h.writeError(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    h.writeResponse(w, resp)
}

// writeResponse writes a successful response
func (h *handler) writeResponse(w http.ResponseWriter, resp *Response) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

// writeError writes an error response
func (h *handler) writeError(w http.ResponseWriter, msg string, status int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(&Response{
        Success: false,
        Error:   msg,
    })
}
```

### 5. Progress Manager (progress/manager.go - ~150 lines)

```go
package progress

import (
    "context"
    "fmt"
    
    tea "github.com/charmbracelet/bubbletea"
)

// Manager manages progress display
type Manager interface {
    Start(ctx context.Context) error
    Stop() error
    Update(status *Status) error
}

// Status represents current execution status
type Status struct {
    DAGName    string
    RunID      string
    TotalSteps int
    CompletedSteps int
    CurrentStep string
    Steps      []*StepStatus
}

// StepStatus represents step status
type StepStatus struct {
    Name   string
    Status execution.Status
    Output string
}

type manager struct {
    program *tea.Program
    model   *progressModel
    enabled bool
}

// NewManager creates a new progress manager
func NewManager(enabled bool) Manager {
    return &manager{
        enabled: enabled,
    }
}

// Start starts the progress display
func (m *manager) Start(ctx context.Context) error {
    if !m.enabled {
        return nil
    }
    
    m.model = newProgressModel()
    m.program = tea.NewProgram(m.model)
    
    go func() {
        if _, err := m.program.Run(); err != nil {
            log.Error("progress display error", "error", err)
        }
    }()
    
    return nil
}

// Stop stops the progress display
func (m *manager) Stop() error {
    if m.program != nil {
        m.program.Quit()
    }
    return nil
}

// Update updates the progress display
func (m *manager) Update(status *Status) error {
    if !m.enabled || m.program == nil {
        return nil
    }
    
    m.program.Send(statusUpdateMsg{status: status})
    return nil
}

// progressModel is the Bubble Tea model
type progressModel struct {
    status *Status
}

func newProgressModel() *progressModel {
    return &progressModel{}
}

func (m *progressModel) Init() tea.Cmd {
    return nil
}

func (m *progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case statusUpdateMsg:
        m.status = msg.status
        return m, nil
    case tea.KeyMsg:
        if msg.String() == "q" {
            return m, tea.Quit
        }
    }
    return m, nil
}

func (m *progressModel) View() string {
    if m.status == nil {
        return "Initializing..."
    }
    
    // Build progress display
    var view string
    view += fmt.Sprintf("DAG: %s (Run: %s)\n", m.status.DAGName, m.status.RunID)
    view += fmt.Sprintf("Progress: %d/%d steps\n\n", m.status.CompletedSteps, m.status.TotalSteps)
    
    for _, step := range m.status.Steps {
        icon := getStatusIcon(step.Status)
        view += fmt.Sprintf("%s %s\n", icon, step.Name)
    }
    
    return view
}

type statusUpdateMsg struct {
    status *Status
}

func getStatusIcon(status execution.Status) string {
    switch status {
    case execution.StatusRunning:
        return "⏳"
    case execution.StatusSuccess:
        return "✅"
    case execution.StatusError:
        return "❌"
    default:
        return "⏺"
    }
}
```

## Benefits of Decomposition

### 1. Clear Separation of Concerns
- Execution logic in `executor`
- Status reporting in `reporter`
- Socket communication in `socket`
- Progress display in `progress`

### 2. Improved Testability
```go
// Easy to mock each component
func TestAgent_Run(t *testing.T) {
    mockExecutor := NewMockExecutor(t)
    mockReporter := NewMockReporter(t)
    mockSocket := NewMockSocket(t)
    mockProgress := NewMockProgress(t)
    
    agent := New(mockExecutor, mockReporter, mockSocket, mockProgress, &Config{})
    
    // Test agent orchestration
    err := agent.Run(context.Background())
    require.NoError(t, err)
}
```

### 3. Easier Maintenance
- Each component is ~150-300 lines
- Single responsibility per component
- Clear interfaces

### 4. Better Extensibility
- Easy to add new execution strategies
- Simple to add new reporters (Slack, PagerDuty, etc.)
- Support for different progress displays

## Conclusion

This decomposition transforms the 1019-line Agent into focused, testable components while maintaining the same functionality.
