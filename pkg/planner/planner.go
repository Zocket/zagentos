// Package planner 实现 Task Planner。
// P11: 将高层意图分解为 task DAG，支持顺序和并行执行。
package planner

import "context"

// TaskStatus 表示任务状态
type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskInProgress TaskStatus = "in_progress"
	TaskCompleted  TaskStatus = "completed"
	TaskFailed     TaskStatus = "failed"
	TaskSkipped    TaskStatus = "skipped"
)

// Task 表示一个可执行的任务
type Task struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Status       TaskStatus `json:"status"`
	Dependencies []string   `json:"dependencies,omitempty"` // 依赖的任务 ID
	Result       string     `json:"result,omitempty"`
	Error        string     `json:"error,omitempty"`
}

// Plan 是分解后的任务计划
type Plan struct {
	ID    string  `json:"id"`
	Goal  string  `json:"goal"`  // 原始的高层意图
	Tasks []*Task `json:"tasks"`
}

// Planner 将意图分解为任务计划
type Planner interface {
	// Plan 将用户意图分解为 task DAG
	Plan(ctx context.Context, goal string) (*Plan, error)

	// Replan 根据中间结果调整计划
	Replan(ctx context.Context, plan *Plan) (*Plan, error)
}

// Executor 执行任务计划
type Executor interface {
	// Execute 执行一个完整计划，按依赖顺序调度
	Execute(ctx context.Context, plan *Plan) error

	// ExecuteTask 执行单个任务
	ExecuteTask(ctx context.Context, task *Task) error
}

// Scheduler 负责计算任务的执行顺序
type Scheduler interface {
	// Schedule 根据依赖关系计算执行批次
	// 返回值是分批的 task，同一批内的可以并行
	Schedule(plan *Plan) ([][]string, error)
}
