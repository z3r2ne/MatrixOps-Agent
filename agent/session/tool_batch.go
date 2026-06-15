package session

import (
	"sync"

	"matrixops-agent/llm"
	"matrixops-agent/tool"
	"pkgs/db/models"
)

const maxConcurrentToolCalls = 10

type toolCallPlan struct {
	Call llm.ToolCall
	Run  func() (tool.Result, error)
}

type toolCallExecution struct {
	Index  int
	Call   llm.ToolCall
	Result tool.Result
	Err    error
}

func runToolCallPlansInParallel(plans []toolCallPlan, onComplete func(toolCallExecution)) []toolCallExecution {
	results := make([]toolCallExecution, len(plans))
	if len(plans) == 0 {
		return results
	}

	sem := make(chan struct{}, maxConcurrentToolCalls)
	var wg sync.WaitGroup
	for index, plan := range plans {
		index := index
		plan := plan
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			var (
				result tool.Result
				err    error
			)
			if plan.Run != nil {
				result, err = plan.Run()
			}
			execution := toolCallExecution{
				Index:  index,
				Call:   plan.Call,
				Result: result,
				Err:    err,
			}
			results[execution.Index] = execution
			if onComplete != nil {
				onComplete(execution)
			}
		}()
	}

	wg.Wait()
	return results
}

func (r *AgentRunner) prepareToolCallPlan(runtimeConfig *RuntimeConfig, sess *Info, worker *models.Worker, call llm.ToolCall) toolCallPlan {
	return r.prepareToolCallPlanWithContext(runtimeConfig, sess, worker, call, tool.Context{})
}

func (r *AgentRunner) prepareToolCallPlanWithContext(runtimeConfig *RuntimeConfig, sess *Info, worker *models.Worker, call llm.ToolCall, execCtx tool.Context) toolCallPlan {
	toolInstance, err := runtimeConfig.ToolRegistry.Get(call.Name)
	if err != nil {
		return toolCallPlan{
			Call: call,
			Run: func() (tool.Result, error) {
				return tool.Result{IsError: true, Name: call.Name}, err
			},
		}
	}

	if err := r.authorizeToolCall(runtimeConfig, worker, call, toolInstance); err != nil {
		if blockedResult, ok := blockedToolResult(toolInstance, err); ok {
			return toolCallPlan{
				Call: call,
				Run: func() (tool.Result, error) {
					return blockedResult, nil
				},
			}
		}

		return toolCallPlan{
			Call: call,
			Run: func() (tool.Result, error) {
				return tool.Result{IsError: true, Name: toolInstance.Name()}, err
			},
		}
	}

	ctx := execCtx
	if ctx.SessionID == "" {
		ctx.SessionID = sess.ID
	}
	if ctx.Directory == "" {
		ctx.Directory = sess.Directory
	}
	if ctx.Worktree == "" {
		ctx.Worktree = sess.Directory
	}
	if ctx.Context == nil {
		ctx.Context = runtimeConfig.Ctx
	}
	toolExecCtx, _, cleanup := tool.DeriveToolCallContext(ctx.Context, call.ID, toolInstance.Name())
	ctx.Context = toolExecCtx

	return toolCallPlan{
		Call: call,
		Run: func() (tool.Result, error) {
			defer cleanup()
			result, err := tool.ExecuteWithOutputTruncation(toolInstance, ctx, call.Arguments)
			if err != nil {
				if result.Name == "" {
					result.Name = toolInstance.Name()
				}
				result.IsError = true
				return result, err
			}
			if isPlanMutatingTool(toolInstance.Name()) {
				r.emitPlanUpdated()
			}
			return result, nil
		},
	}
}

func blockedToolResult(toolInstance tool.Tool, err error) (tool.Result, bool) {
	result := tool.Result{
		Name:     toolInstance.Name(),
		Metadata: map[string]interface{}{"blocked": true},
	}

	switch typed := err.(type) {
	case WorkerToolDisabledError:
		result.Content = "[tool call]: worker has not enabled " + typed.ToolName
		result.Metadata["blockedReason"] = "worker_disabled"
		return result, true
	case ProjectToolDeniedError:
		result.Content = "[tool call]: project denied " + typed.ToolName
		result.Metadata["blockedReason"] = "project_denied"
		return result, true
	case ProjectToolRejectedError:
		result.Content = "[tool call]: user rejected " + typed.ToolName
		result.Metadata["blockedReason"] = "user_rejected"
		if typed.Reason != "" {
			result.Content += ", reason " + typed.Reason
			result.Metadata["reason"] = typed.Reason
		}
		return result, true
	default:
		return tool.Result{}, false
	}
}

func isPermissionError(err error) bool {
	switch err.(type) {
	case WorkerToolDisabledError, ProjectToolDeniedError, ProjectToolRejectedError:
		return true
	default:
		return false
	}
}
