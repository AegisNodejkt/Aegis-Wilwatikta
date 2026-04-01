package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type ParallelExecutor struct {
	orchestrator *PipelineOrchestrator
}

func NewParallelExecutor(config PipelineConfig) *ParallelExecutor {
	return &ParallelExecutor{
		orchestrator: NewPipelineOrchestrator(config),
	}
}

func (e *ParallelExecutor) ExecuteParallel(ctx context.Context, tasks []ParallelTask) ([]*ParallelTaskResult, error) {
	var wg sync.WaitGroup
	results := make([]*ParallelTaskResult, len(tasks))
	errCh := make(chan error, len(tasks))

	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t ParallelTask) {
			defer wg.Done()

			startTime := time.Now()
			output, err := t.Executor.Execute(ctx, t.Input)
			duration := time.Since(startTime)

			results[idx] = &ParallelTaskResult{
				Name:     t.Name,
				Output:   output,
				Error:    err,
				Duration: duration,
			}

			if err != nil {
				errCh <- fmt.Errorf("task %s failed: %w", t.Name, err)
			}
		}(i, task)
	}

	wg.Wait()
	close(errCh)

	var errors []error
	for err := range errCh {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return results, fmt.Errorf("parallel execution had %d error(s)", len(errors))
	}

	return results, nil
}

type ParallelTask struct {
	Name     string
	Executor AgentExecutor
	Input    interface{}
}

type ParallelTaskResult struct {
	Name     string
	Output   interface{}
	Error    error
	Duration time.Duration
}

func (e *ParallelExecutor) ExecuteScoutAndRAG(ctx context.Context, scout AgentExecutor, ragExecutor AgentExecutor, input *PipelineInput) (*ScoutOutput, interface{}, error) {
	tasks := []ParallelTask{
		{
			Name:     "scout",
			Executor: scout,
			Input:    input,
		},
	}

	if ragExecutor != nil {
		tasks = append(tasks, ParallelTask{
			Name:     "rag",
			Executor: ragExecutor,
			Input:    input,
		})
	}

	results, err := e.ExecuteParallel(ctx, tasks)
	if err != nil {
		return nil, nil, err
	}

	var scoutOutput *ScoutOutput
	var ragOutput interface{}

	for _, result := range results {
		if result.Name == "scout" {
			if result.Output != nil {
				scoutOutput = result.Output.(*ScoutOutput)
			}
		} else if result.Name == "rag" {
			ragOutput = result.Output
		}
	}

	return scoutOutput, ragOutput, nil
}
