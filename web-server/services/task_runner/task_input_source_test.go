package task_runner

import (
	"testing"

	"pkgs/db/models"
)

func TestQueueItemInputSource(t *testing.T) {
	if got := QueueItemInputSource(nil); got != TaskInputSourceFrontend {
		t.Fatalf("nil item = %q, want frontend", got)
	}
	if got := QueueItemInputSource(&models.TaskMessageQueueItem{
		Source: TaskInputSourceWeChat,
	}); got != TaskInputSourceWeChat {
		t.Fatalf("source field = %q, want wechat", got)
	}
	if got := QueueItemInputSource(&models.TaskMessageQueueItem{
		Metadata: map[string]interface{}{"source": TaskInputSourceWeChat},
	}); got != TaskInputSourceWeChat {
		t.Fatalf("metadata source = %q, want wechat", got)
	}
	if got := QueueItemInputSource(&models.TaskMessageQueueItem{
		ID: "wechat-bot-1",
	}); got != TaskInputSourceWeChat {
		t.Fatalf("wechat id prefix = %q, want wechat", got)
	}
	if got := QueueItemInputSource(&models.TaskMessageQueueItem{
		ID: "queue-123",
	}); got != TaskInputSourceFrontend {
		t.Fatalf("default = %q, want frontend", got)
	}
}

func TestInputSourceForwardsToWeChat(t *testing.T) {
	if inputSourceForwardsToWeChat(TaskInputSourceFrontend) {
		t.Fatal("frontend should not forward")
	}
	if !inputSourceForwardsToWeChat(TaskInputSourceWeChat) {
		t.Fatal("wechat should forward")
	}
}
