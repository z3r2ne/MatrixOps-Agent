package coreagent

import (
	"matrixops.local/core_agent/streamtypes"
)

// Re-export all streamtypes so that existing code in the coreagent package
// and external packages can continue to use coreagent.XXX without modification.

type ModelMessage = streamtypes.ModelMessage
type ChatRequest = streamtypes.ChatRequest
type ChatResponse = streamtypes.ChatResponse
type StreamEvent = streamtypes.StreamEvent
type Usage = streamtypes.Usage
type ToolDefinition = streamtypes.ToolDefinition
type ToolCall = streamtypes.ToolCall
type ToolContext = streamtypes.ToolContext
type ActionOutput = streamtypes.ActionOutput
type CallToolRequest = streamtypes.CallToolRequest
type CompatibleControlHandler = streamtypes.CompatibleControlHandler
type StreamInput = streamtypes.StreamInput
type StreamOutput = streamtypes.StreamOutput
type ActionPromptSchema = streamtypes.ActionPromptSchema
type StreamingActionBuffer = streamtypes.StreamingActionBuffer
type RetryRawBuffer = streamtypes.RetryRawBuffer

// Interfaces
type ChatClient = streamtypes.ChatClient
type StreamChatClient = streamtypes.StreamChatClient
type StreamChatClientWithOptions = streamtypes.StreamChatClientWithOptions

// Options
type StreamChatOptions = streamtypes.StreamChatOptions
type StreamChatOption = streamtypes.StreamChatOption

var NewStreamChatOptions = streamtypes.NewStreamChatOptions
var WithOnRequest = streamtypes.WithOnRequest
var WithOnRawRequest = streamtypes.WithOnRawRequest
var WithOnRawResponse = streamtypes.WithOnRawResponse
var WithHTTPClient = streamtypes.WithHTTPClient
var WithOnRetryError = streamtypes.WithOnRetryError

// Utils
var RenderContent = streamtypes.RenderContent
var TruncateStringForLog = streamtypes.TruncateStringForLog
var RawResponseLooksLikeRetryableProxyHTML = streamtypes.RawResponseLooksLikeRetryableProxyHTML
var IsWhitespaceByte = streamtypes.IsWhitespaceByte
var SnippetAroundBytes = streamtypes.SnippetAroundBytes
var FirstNonWhitespaceByte = streamtypes.FirstNonWhitespaceByte

// Buffer
var NewStreamingActionBuffer = streamtypes.NewStreamingActionBuffer

// Retry
var StreamWithRetries = streamtypes.StreamWithRetries
var StreamShouldRetryError = streamtypes.StreamShouldRetryError
var StreamShouldRetryParseError = streamtypes.StreamShouldRetryParseError
var StreamMaxRetries = streamtypes.StreamMaxRetries
var StreamRetryDelayForError = streamtypes.StreamRetryDelayForError
var StreamSleepWithContext = streamtypes.StreamSleepWithContext
var AbortDone = streamtypes.AbortDone
var RetryableHTTPStatus = streamtypes.RetryableHTTPStatus
var NewRetryRawBuffer = streamtypes.NewRetryRawBuffer

// Parse
var ParseActionBytes = streamtypes.ParseActionBytes
var DecodeActionOutput = streamtypes.DecodeActionOutput
var FlatToolNameParamsToActionOutput = streamtypes.FlatToolNameParamsToActionOutput
var ParseActionStream = streamtypes.ParseActionStream
