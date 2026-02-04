package scheduler

import (
  "fmt"
  "time"
)

type Workflow struct {
  ID          string         `yaml:"id" json:"id"`
  Name        string         `yaml:"name" json:"name"`
  Description string         `yaml:"description" json:"description"`
  Steps       []WorkflowStep `yaml:"steps" json:"steps"`
  OnError     string         `yaml:"on_error" json:"on_error"` // stop, continue
  Timeout     time.Duration  `yaml:"timeout" json:"timeout"`
}

type WorkflowStep struct {
  ID        string     `yaml:"id" json:"id"`
  Name      string     `yaml:"name" json:"name"`
  Type      TaskType   `yaml:"type" json:"type"`
  Config    TaskConfig `yaml:"config" json:"config"`
  DependsOn []string   `yaml:"depends_on" json:"depends_on"`
  Condition string     `yaml:"condition" json:"condition"`
  OnError   string     `yaml:"on_error" json:"on_error"`
}

type WorkflowResult struct {
  WorkflowID string
  Success    bool
  Steps      []WorkflowStepResult
  StartTime  time.Time
  EndTime    time.Time
  Error      string
}

type WorkflowStepResult struct {
  StepID   string
  Success  bool
  Output   string
  Error    string
  Skipped  bool
  Duration time.Duration
}

type WorkflowEngine struct {
  workflows map[string]*Workflow
  executor  TaskExecutor
}

func NewWorkflowEngine(executor TaskExecutor) *WorkflowEngine {
  return &WorkflowEngine{
    workflows: make(map[string]*Workflow),
    executor:  executor,
  }
}

func (e *WorkflowEngine) Register(workflow *Workflow) {
  e.workflows[workflow.ID] = workflow
}

func (e *WorkflowEngine) Get(id string) (*Workflow, bool) {
  w, ok := e.workflows[id]
  return w, ok
}

func (e *WorkflowEngine) Execute(workflowID string) (*WorkflowResult, error) {
  workflow, ok := e.workflows[workflowID]
  if !ok {
    return nil, fmt.Errorf("workflow %s not found", workflowID)
  }

  result := &WorkflowResult{
    WorkflowID: workflowID,
    StartTime:  time.Now(),
    Success:    true,
  }

  completed := make(map[string]bool)
  stepResults := make(map[string]*WorkflowStepResult)

  for _, step := range workflow.Steps {
    // Check dependencies
    for _, dep := range step.DependsOn {
      if !completed[dep] {
        result.Success = false
        result.Error = fmt.Sprintf("step %s depends on incomplete step %s", step.ID, dep)
        result.EndTime = time.Now()
        return result, nil
      }
      if depResult, ok := stepResults[dep]; ok && !depResult.Success {
        // Skip if dependency failed
        stepResult := WorkflowStepResult{
          StepID:  step.ID,
          Skipped: true,
        }
        result.Steps = append(result.Steps, stepResult)
        continue
      }
    }

    // Execute step
    stepResult := e.executeStep(&step)
    result.Steps = append(result.Steps, stepResult)
    stepResults[step.ID] = &stepResult
    completed[step.ID] = true

    if !stepResult.Success {
      if step.OnError == "stop" || (step.OnError == "" && workflow.OnError == "stop") {
        result.Success = false
        result.Error = fmt.Sprintf("step %s failed: %s", step.ID, stepResult.Error)
        break
      }
    }
  }

  result.EndTime = time.Now()
  return result, nil
}

func (e *WorkflowEngine) executeStep(step *WorkflowStep) WorkflowStepResult {
  startTime := time.Now()
  result := WorkflowStepResult{
    StepID: step.ID,
  }

  task := &ScheduledTask{
    ID:     step.ID,
    Name:   step.Name,
    Type:   step.Type,
    Config: step.Config,
  }

  taskResult, err := e.executor.ExecuteTask(task)
  if err != nil {
    result.Success = false
    result.Error = err.Error()
  } else {
    result.Success = taskResult.Success
    result.Output = taskResult.Output
    if !taskResult.Success {
      result.Error = taskResult.Error
    }
  }

  result.Duration = time.Since(startTime)
  return result
}
