package types

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// MessageKind 区分用户主动输入与系统注入消息（UI 展示；默认空或 user 均视作用户消息）。
const (
	MessageKindUser   = "user"
	MessageKindSystem = "system"
)

const (
	PartTypeText               = "text"
	PartTypeReasoning          = "reasoning"
	PartTypeTool               = "tool"
	PartTypeFinish             = "finish"
	PartTypeError              = "error"
	PartTypeStartStep          = "start-step"
	PartTypeFinishStep         = "finish-step"
	PartTypeTextDelta          = "text-delta"
	PartTypeReasoningDelta     = "reasoning-delta"
	PartTypeToolDelta          = "tool-delta"
	PartTypeCompaction         = "compaction"
	PartTypeMemoryOrganization = "memory-organization"
	PartTypePatch              = "patch"
)

// MessageFooterStatus 助手消息底部一行状态（仅实时推送展示，可不落库）
type MessageFooterStatus struct {
	Text    string `json:"text"`
	Loading bool   `json:"loading"`
}

type MessageInfo struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionID"`
	Role      Role   `json:"role"`
	// MessageKind 为 system 时表示系统/补充类消息，前端需与用户消息区分展示。
	MessageKind string `json:"messageKind,omitempty"`
	// MessageOrigin 标识系统消息来源（如 reminder、stall_watchdog）。
	MessageOrigin string `json:"messageOrigin,omitempty"`
	ParentID  string `json:"parentID,omitempty"`
	// Mode       string          `json:"mode,omitempty"`
	Occupation   string               `json:"occupation,omitempty"`
	Worker       string               `json:"worker,omitempty"`
	Provider     string               `json:"provider,omitempty"`
	Model        string               `json:"model,omitempty"`
	ProviderID   string               `json:"providerID,omitempty"`
	ModelID      string               `json:"modelID,omitempty"`
	System       string               `json:"system,omitempty"`
	Tools        map[string]bool      `json:"tools,omitempty"`
	Variant      string               `json:"variant,omitempty"`
	Summary      interface{}          `json:"summary,omitempty"`
	Finish       string               `json:"finish,omitempty"`
	Cost         float64              `json:"cost,omitempty"`
	Memory       *Memory              `json:"memory,omitempty"`
	Tokens       *MessageTokens       `json:"tokens,omitempty"`
	Error        *MessageError        `json:"error,omitempty"`
	Path         *MessagePath         `json:"path,omitempty"`
	Time         MessageTime          `json:"time"`
	State        string               `json:"state,omitempty"`
	Snapshot     string               `json:"snapshot,omitempty"`
	FooterStatus *MessageFooterStatus `json:"footerStatus,omitempty"`
	Phase                      string   `json:"phase,omitempty"`
	ResponsesOutputMessageRaw  string   `json:"responsesOutputMessageRaw,omitempty"`
	ResponsesReasoningItemRaws []string `json:"responsesReasoningItemRaws,omitempty"`
}

type MessagePath struct {
	Cwd  string `json:"cwd"`
	Root string `json:"root"`
}

type MessageTime struct {
	Created   int64 `json:"created"`
	Completed int64 `json:"completed,omitempty"`
}

type Part struct {
	ID          string                 `json:"id"`
	MessageID   string                 `json:"messageID"`
	SessionID   string                 `json:"sessionID"`
	Type        string                 `json:"type"`
	Text        string                 `json:"text,omitempty"`
	Tool        *ToolPart              `json:"tool,omitempty"`
	Reasoning   string                 `json:"reasoning,omitempty"`
	Synthetic   bool                   `json:"synthetic,omitempty"`
	Ignored     bool                   `json:"ignored,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Snapshot    string                 `json:"snapshot,omitempty"`
	Hash        string                 `json:"hash,omitempty"`
	Files       []string               `json:"files,omitempty"`
	Mime        string                 `json:"mime,omitempty"`
	Filename    string                 `json:"filename,omitempty"`
	Path            string                 `json:"path,omitempty"`
	InputSource     string                 `json:"inputSource,omitempty"`
	URL             string                 `json:"url,omitempty"`
	Source          interface{}            `json:"source,omitempty"`
	AgentName   string                 `json:"name,omitempty"`
	Auto        bool                   `json:"auto,omitempty"`
	Description string                 `json:"description,omitempty"`
	Subagent    string                 `json:"agent,omitempty"`
	Model       *ModelRef              `json:"model,omitempty"`
	Command     string                 `json:"command,omitempty"`
	Attempt     int                    `json:"attempt,omitempty"`
	Error       *MessageError          `json:"error,omitempty"`
	Reason      string                 `json:"reason,omitempty"`
	Cost        float64                `json:"cost,omitempty"`
	Tokens      *MessageTokens         `json:"tokens,omitempty"`
	Time        *PartTime              `json:"time,omitempty"`
}

type PartTime struct {
	Start     int64 `json:"start,omitempty"`
	End       int64 `json:"end,omitempty"`
	Created   int64 `json:"created,omitempty"`
	Compacted int64 `json:"compacted,omitempty"`
}

type ToolPart struct {
	Name     string                 `json:"tool"`
	CallID   string                 `json:"callID"`
	State    ToolState              `json:"state"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type ToolState struct {
	Status         string                 `json:"status"`
	Input          interface{}            `json:"input,omitempty"`
	Raw            string                 `json:"raw,omitempty"`
	Title          string                 `json:"title,omitempty"`
	SystemMessage  string                 `json:"systemMessage,omitempty"`
	Output         string                 `json:"output,omitempty"`
	Error          string                 `json:"error,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Attachments    []Part                 `json:"attachments,omitempty"`
	Time           PartTime               `json:"time"`
	MemoryMetadata map[string]interface{} `json:"-"`
	FullOutput     string                 `json:"-"`
}

type WithParts struct {
	Info  *MessageInfo `json:"info"`
	Parts []*Part      `json:"parts"`
}

type MessageTokens struct {
	Input     int        `json:"input"`
	Output    int        `json:"output"`
	Reasoning int        `json:"reasoning"`
	Cache     TokenCache `json:"cache"`
}

type TokenCache struct {
	Read  int `json:"read"`
	Write int `json:"write"`
}

type MessageError struct {
	Name            string            `json:"name"`
	Message         string            `json:"message,omitempty"`
	ProviderID      string            `json:"providerID,omitempty"`
	StatusCode      int               `json:"statusCode,omitempty"`
	IsRetryable     bool              `json:"isRetryable,omitempty"`
	ResponseBody    string            `json:"responseBody,omitempty"`
	ResponseHeaders map[string]string `json:"responseHeaders,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

type TextRange struct {
	Value string `json:"value"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

type LSPPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type LSPRange struct {
	Start LSPPosition `json:"start"`
	End   LSPPosition `json:"end"`
}

type FilePartSource struct {
	Type       string     `json:"type"`
	Path       string     `json:"path,omitempty"`
	Range      *LSPRange  `json:"range,omitempty"`
	Name       string     `json:"name,omitempty"`
	Kind       int        `json:"kind,omitempty"`
	ClientName string     `json:"clientName,omitempty"`
	URI        string     `json:"uri,omitempty"`
	Text       *TextRange `json:"text,omitempty"`
}

type StreamMessageType string

// switch (value.type) {
// case "start":
//   SessionStatus.set(input.sessionID, { type: "busy" })
//   break

// case "reasoning-start":
//   if (value.id in reasoningMap) {
// 	continue
//   }
//   reasoningMap[value.id] = {
// 	id: Identifier.ascending("part"),
// 	messageID: input.assistantMessage.id,
// 	sessionID: input.assistantMessage.sessionID,
// 	type: "reasoning",
// 	text: "",
// 	time: {
// 	  start: Date.now(),
// 	},
// 	metadata: value.providerMetadata,
//   }
//   break

// case "reasoning-delta":
//   if (value.id in reasoningMap) {
// 	const part = reasoningMap[value.id]
// 	part.text += value.text
// 	if (value.providerMetadata) part.metadata = value.providerMetadata
// 	if (part.text) await Session.updatePart({ part, delta: value.text })
//   }
//   break

// case "reasoning-end":
//   if (value.id in reasoningMap) {
// 	const part = reasoningMap[value.id]
// 	part.text = part.text.trimEnd()

// 	part.time = {
// 	  ...part.time,
// 	  end: Date.now(),
// 	}
// 	if (value.providerMetadata) part.metadata = value.providerMetadata
// 	await Session.updatePart(part)
// 	delete reasoningMap[value.id]
//   }
//   break

// case "tool-input-start":
//   const part = await Session.updatePart({
// 	id: toolcalls[value.id]?.id ?? Identifier.ascending("part"),
// 	messageID: input.assistantMessage.id,
// 	sessionID: input.assistantMessage.sessionID,
// 	type: "tool",
// 	tool: value.toolName,
// 	callID: value.id,
// 	state: {
// 	  status: "pending",
// 	  input: {},
// 	  raw: "",
// 	},
//   })
//   toolcalls[value.id] = part as MessageV2.ToolPart
//   break

// case "tool-input-delta":
//   break

// case "tool-input-end":
//   break

// case "tool-call": {
//   const match = toolcalls[value.toolCallId]
//   if (match) {
// 	const part = await Session.updatePart({
// 	  ...match,
// 	  tool: value.toolName,
// 	  state: {
// 		status: "running",
// 		input: value.input,
// 		time: {
// 		  start: Date.now(),
// 		},
// 	  },
// 	  metadata: value.providerMetadata,
// 	})
// 	toolcalls[value.toolCallId] = part as MessageV2.ToolPart

// 	const parts = await MessageV2.parts(input.assistantMessage.id)
// 	const lastThree = parts.slice(-DOOM_LOOP_THRESHOLD)

// 	if (
// 	  lastThree.length === DOOM_LOOP_THRESHOLD &&
// 	  lastThree.every(
// 		(p) =>
// 		  p.type === "tool" &&
// 		  p.tool === value.toolName &&
// 		  p.state.status !== "pending" &&
// 		  JSON.stringify(p.state.input) === JSON.stringify(value.input),
// 	  )
// 	) {
// 	  const agent = await Agent.get(input.assistantMessage.agent)
// 	  await PermissionNext.ask({
// 		permission: "doom_loop",
// 		patterns: [value.toolName],
// 		sessionID: input.assistantMessage.sessionID,
// 		metadata: {
// 		  tool: value.toolName,
// 		  input: value.input,
// 		},
// 		always: [value.toolName],
// 		ruleset: agent.permission,
// 	  })
// 	}
//   }
//   break
// }
// case "tool-result": {
//   const match = toolcalls[value.toolCallId]
//   if (match && match.state.status === "running") {
// 	await Session.updatePart({
// 	  ...match,
// 	  state: {
// 		status: "completed",
// 		input: value.input ?? match.state.input,
// 		output: value.output.output,
// 		metadata: value.output.metadata,
// 		title: value.output.title,
// 		time: {
// 		  start: match.state.time.start,
// 		  end: Date.now(),
// 		},
// 		attachments: value.output.attachments,
// 	  },
// 	})

// 	delete toolcalls[value.toolCallId]
//   }
//   break
// }

// case "tool-error": {
//   const match = toolcalls[value.toolCallId]
//   if (match && match.state.status === "running") {
// 	await Session.updatePart({
// 	  ...match,
// 	  state: {
// 		status: "error",
// 		input: value.input ?? match.state.input,
// 		error: (value.error as any).toString(),
// 		time: {
// 		  start: match.state.time.start,
// 		  end: Date.now(),
// 		},
// 	  },
// 	})

// 	if (
// 	  value.error instanceof PermissionNext.RejectedError ||
// 	  value.error instanceof Question.RejectedError
// 	) {
// 	  blocked = shouldBreak
// 	}
// 	delete toolcalls[value.toolCallId]
//   }
//   break
// }
// case "error":
//   throw value.error

// case "start-step":
//   snapshot = await Snapshot.track()
//   await Session.updatePart({
// 	id: Identifier.ascending("part"),
// 	messageID: input.assistantMessage.id,
// 	sessionID: input.sessionID,
// 	snapshot,
// 	type: "step-start",
//   })
//   break

// case "finish-step":
//   const usage = Session.getUsage({
// 	model: input.model,
// 	usage: value.usage,
// 	metadata: value.providerMetadata,
//   })
//   input.assistantMessage.finish = value.finishReason
//   input.assistantMessage.cost += usage.cost
//   input.assistantMessage.tokens = usage.tokens
//   await Session.updatePart({
// 	id: Identifier.ascending("part"),
// 	reason: value.finishReason,
// 	snapshot: await Snapshot.track(),
// 	messageID: input.assistantMessage.id,
// 	sessionID: input.assistantMessage.sessionID,
// 	type: "step-finish",
// 	tokens: usage.tokens,
// 	cost: usage.cost,
//   })
//   await Session.updateMessage(input.assistantMessage)
//   if (snapshot) {
// 	const patch = await Snapshot.patch(snapshot)
// 	if (patch.files.length) {
// 	  await Session.updatePart({
// 		id: Identifier.ascending("part"),
// 		messageID: input.assistantMessage.id,
// 		sessionID: input.sessionID,
// 		type: "patch",
// 		hash: patch.hash,
// 		files: patch.files,
// 	  })
// 	}
// 	snapshot = undefined
//   }
//   SessionSummary.summarize({
// 	sessionID: input.sessionID,
// 	messageID: input.assistantMessage.parentID,
//   })
//   if (await SessionCompaction.isOverflow({ tokens: usage.tokens, model: input.model })) {
// 	needsCompaction = true
//   }
//   break

// case "text-start":
//   currentText = {
// 	id: Identifier.ascending("part"),
// 	messageID: input.assistantMessage.id,
// 	sessionID: input.assistantMessage.sessionID,
// 	type: "text",
// 	text: "",
// 	time: {
// 	  start: Date.now(),
// 	},
// 	metadata: value.providerMetadata,
//   }
//   break

// case "text-delta":
//   if (currentText) {
// 	currentText.text += value.text
// 	if (value.providerMetadata) currentText.metadata = value.providerMetadata
// 	if (currentText.text)
// 	  await Session.updatePart({
// 		part: currentText,
// 		delta: value.text,
// 	  })
//   }
//   break

// case "text-end":
//   if (currentText) {
// 	currentText.text = currentText.text.trimEnd()
// 	const textOutput = await Plugin.trigger(
// 	  "experimental.text.complete",
// 	  {
// 		sessionID: input.sessionID,
// 		messageID: input.assistantMessage.id,
// 		partID: currentText.id,
// 	  },
// 	  { text: currentText.text },
// 	)
// 	currentText.text = textOutput.text
// 	currentText.time = {
// 	  start: Date.now(),
// 	  end: Date.now(),
// 	}
// 	if (value.providerMetadata) currentText.metadata = value.providerMetadata
// 	await Session.updatePart(currentText)
//   }
//   currentText = undefined
//   break

// case "finish":
//   break

// default:
//
//	  log.info("unhandled", {
//		...value,
//	  })
//	  continue
//	}
const (
	StreamMessageTypeStart          StreamMessageType = "start"
	StreamMessageTypeReasoningStart StreamMessageType = "reasoning-start"
	StreamMessageTypeReasoningDelta StreamMessageType = "reasoning-delta"
	StreamMessageTypeReasoningEnd   StreamMessageType = "reasoning-end"
	StreamMessageTypeToolInputStart StreamMessageType = "tool-input-start"
	StreamMessageTypeToolInputDelta StreamMessageType = "tool-input-delta"
	StreamMessageTypeToolInputEnd   StreamMessageType = "tool-input-end"
	StreamMessageTypeToolCall       StreamMessageType = "tool-call"
	StreamMessageTypeToolResult     StreamMessageType = "tool-result"
	StreamMessageTypeToolError      StreamMessageType = "tool-error"
	StreamMessageTypeError          StreamMessageType = "error"
	StreamMessageTypeFinish         StreamMessageType = "start-step"
	StreamMessageTypeFinishStep     StreamMessageType = "finish-step"
	StreamMessageTypeStartStep      StreamMessageType = "text-start"
	StreamMessageTypeTextDelta      StreamMessageType = "text-delta"
	StreamMessageTypeTextEnd        StreamMessageType = "text-end"
	StreamMessageTypeTextFinish     StreamMessageType = "text-finish"
)
