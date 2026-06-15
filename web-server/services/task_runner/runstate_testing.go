package task_runner

// MarkTaskRunningForTest 标记任务为运行中，用于测试入队 / send-next 等分支。
func MarkTaskRunningForTest(taskID uint) {
	addTaskRuntime(&TaskRuntime{taskID: taskID})
}

// UnmarkTaskRunningForTest 清除测试用的运行中标记。
func UnmarkTaskRunningForTest(taskID uint) {
	removeTaskRuntime(taskID)
}
