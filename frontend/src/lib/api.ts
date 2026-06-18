import { httpClient, RequestConfig, API_BASE_URL as HTTP_CLIENT_BASE_URL } from './httpClient';

// API 基础配置
export const API_BASE_URL = HTTP_CLIENT_BASE_URL;
const API_ROOT = API_BASE_URL.replace(/\/api\/?$/, '');

// Workspace 类型定义
export interface Project {
  id: number;
  name: string;
  path: string;
  icon: string;
  color: string;
  status: string;
  activeTasks: number;
  prompt?: string;
  toolPermissions: string;
  memoryLibraryIds?: number[];
  yoloMode: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface ProjectResponse extends Project {
  pathExists: boolean;
  error?: string;
}

export interface ProjectCreate {
  name: string;
  path?: string;
  createNew?: boolean;
  newPath?: string;
  icon?: string;
  color?: string;
  toolPermissions?: string;
  memoryLibraryIds?: number[];
  yoloMode?: boolean;
}

export type TaskMemoryLibraryMode = 'none' | 'temporary' | 'libraries';

export interface MemoryLibrary {
  id: number;
  name: string;
  content: string;
  isRag?: boolean;
  isTemporary?: boolean;
  taskId?: number;
  createdAt: string;
  updatedAt: string;
}

export interface MemoryLibraryCreate {
  name: string;
  content?: string;
  isRag?: boolean;
}

export interface MemoryLibraryUpdate {
  name?: string;
  content?: string;
}

export interface Provider {
  name: string;
  displayName: string;
  type: string;
  enabled: boolean;
  detected: boolean;
  path?: string;
  status: string;
  message?: string;
}

export interface GlobalConfig {
  id: number;
  key: string;
  value: string;
  createdAt: string;
  updatedAt: string;
}

export interface ShellInfo {
  id: string;
  name: string;
  command: string;
  path?: string;
  isAvailable: boolean;
  isCurrent?: boolean;
  isCustom?: boolean;
}

export interface CurrentShellResponse {
  current: ShellInfo;
  options: ShellInfo[];
}

export interface Worker {
  id: number;
  name: string;
  provider: string;
  model: string;
  modelSettingsName?: string;
  description: string;
  temperature?: number;
  clearTemperature?: boolean;
  systemPrompt: string;
  occupation: string;
  enabledTools: string;
  enabledSkills: string;
  llmConfigId?: number;
  workingDir: string;
  createdAt: string;
  updatedAt: string;
}

export interface WorkerCreate {
  name: string;
  provider: string;
  model: string;
  modelSettingsName?: string;
  description: string;
  temperature: number;
  systemPrompt: string;
  occupation: string;
  enabledTools: string;
  enabledSkills: string;
  llmConfigId?: number;
  workingDir: string;
}

export interface WorkerUpdate {
  name?: string;
  provider?: string;
  model?: string;
  modelSettingsName?: string;
  description?: string;
  temperature?: number;
  systemPrompt?: string;
  occupation?: string;
  enabledTools?: string;
  enabledSkills?: string;
  llmConfigId?: number;
  workingDir?: string;
}

export interface WorkerBulkApplyConfigRequest {
  workerIds: number[];
  provider: string;
  model: string;
  modelSettingsName?: string;
  llmConfigId?: number;
}

export interface SkillSource {
  id: number;
  name: string;
  repoUrl: string;
  skillsPath: string;
  enabled: boolean;
  localPath?: string;
  skillCount: number;
  lastSyncAt?: string;
  lastSyncError?: string;
  createdAt: string;
  updatedAt: string;
}

export interface SkillSourceCreate {
  name: string;
  repoUrl: string;
  skillsPath: string;
  enabled?: boolean;
}

export interface SkillSourceUpdate {
  name?: string;
  repoUrl?: string;
  skillsPath?: string;
  enabled?: boolean;
}

export interface SkillCard {
  id: string;
  sourceId: number;
  sourceName: string;
  sourceEnabled: boolean;
  name: string;
  description: string;
  relativePath: string;
  installed: boolean;
  installedPath?: string;
}

export interface ToolInfo {
  name: string;
  verbosName: string;
  description: string;
  isMcp?: boolean;
  mcpServer?: string;
}

export interface McpServer {
  id: number;
  name: string;
  transport: 'stdio' | 'sse' | 'http';
  command?: string;
  argsJson: string;
  envJson: string;
  url?: string;
  headersJson: string;
  enabled: boolean;
  toolCount: number;
  connected: boolean;
  lastConnectError?: string;
  createdAt: string;
  updatedAt: string;
}

export interface McpServerCreate {
  name: string;
  transport?: 'stdio' | 'sse' | 'http';
  command?: string;
  argsJson?: string;
  envJson?: string;
  url?: string;
  headersJson?: string;
  enabled?: boolean;
}

export interface SearchConfig {
  id: number;
  name: string;
  type: 'kimi_search_api';
  apiKey: string;
  baseUrl: string;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface SearchConfigCreate {
  name: string;
  type: 'kimi_search_api';
  apiKey: string;
  baseUrl?: string;
  enabled?: boolean;
}

export interface SearchConfigUpdate {
  name?: string;
  type?: 'kimi_search_api';
  apiKey?: string;
  baseUrl?: string;
  enabled?: boolean;
}

export interface SearchResultItem {
  site_name: string;
  title: string;
  url: string;
  snippet: string;
  content: string;
  date: string;
  icon: string;
  mime: string;
}

export interface SearchTestResponse {
  search_results: SearchResultItem[];
}

export interface EmbeddingConfig {
  id: number;
  name: string;
  type: 'llama_cpp';
  baseUrl: string;
  binaryPath?: string;
  modelPath?: string;
  dimension?: number;
  batchSize?: number;
  maxInputTokens?: number;
  enabled: boolean;
  autoStart?: boolean;
  status?: string;
  lastError?: string;
  createdAt: string;
  updatedAt: string;
}

export interface EmbeddingConfigCreate {
  name: string;
  type: 'llama_cpp';
  baseUrl?: string;
  binaryPath?: string;
  modelPath?: string;
  dimension?: number;
  batchSize?: number;
  maxInputTokens?: number;
  enabled?: boolean;
  autoStart?: boolean;
}

export interface EmbeddingConfigUpdate {
  name?: string;
  type?: 'llama_cpp';
  baseUrl?: string;
  binaryPath?: string;
  modelPath?: string;
  dimension?: number;
  batchSize?: number;
  maxInputTokens?: number;
  enabled?: boolean;
  autoStart?: boolean;
}

export interface EmbeddingTestResponse {
  dimension: number;
  sample?: string;
  vector?: number[];
}

export interface MemoryLibrarySearchIndexStatus {
  memoryLibraryId?: number;
  documentCount: number;
  vectorCount?: number;
  vectorDimension?: number;
  contentBytes?: number;
  indexContentBytes?: number;
  vectorBytes?: number;
  totalBytes?: number;
  status: string;
  progress: number;
  lastError?: string;
  hasIndex: boolean;
}

export interface McpServerUpdate {
  name?: string;
  transport?: 'stdio' | 'sse' | 'http';
  command?: string;
  argsJson?: string;
  envJson?: string;
  url?: string;
  headersJson?: string;
  enabled?: boolean;
}

export interface McpToolInfo {
  name: string;
  fullName: string;
  serverName: string;
  description: string;
  inputSchema?: Record<string, unknown>;
}

export interface Task {
  id: number;
  workspaceId: number;
  projectId: number;
  parentTaskId?: number;
  name?: string;
  sessionTitle?: string;
  content: string;
  memo?: string;
  workerId?: number;
  workerName?: string;
  status: 'queue' | 'running' | 'done' | 'failed' | 'cancelled';
  projectName?: string;
  branch?: string;
  /** 创建任务时选择的基准分支（用于「对比 base」） */
  baseBranch?: string;
  /** 创建任务时基分支对应的提交哈希；旧任务可能为空 */
  baseCommitHash?: string;
  workDir?: string;
  sessionId?: string;
  promptCacheKey?: string;
  /** 工作区内排序，越小越靠前（后端 list_position） */
  listPosition?: number;
  memoryLibraryMode?: TaskMemoryLibraryMode;
  memoryLibraryIds?: number[];
  createdAt: string;
  updatedAt: string;
}

export interface TaskCreate {
  name?: string;
  /** 可为空：仅创建任务、稍后在对话题中输入 */
  content?: string;
  projectId?: number;
  workerId?: number;
  workerName?: string;
  parentTaskId?: number;
  branch?: string;
  newBranch?: string;
  baseBranch?: string;
  inputParts?: Array<{
    type: 'text' | 'file';
    text?: string;
    path?: string;
    url?: string;
    mime?: string;
    filename?: string;
      inputSource?: 'paste' | 'picker' | 'drop';
  }>;
  memoryLibraryMode?: TaskMemoryLibraryMode;
  memoryLibraryIds?: number[];
}

export interface TaskMessageQueueItem {
  id: string;
  type?: 'user' | 'system' | 'append' | 'memory_compaction' | string;
  content: string;
  source?: string;
  supplement?: boolean;
  parts?: Array<{
    type: 'text' | 'file';
    text?: string;
    path?: string;
    url?: string;
    mime?: string;
    filename?: string;
    inputSource?: string;
  }>;
  metadata?: Record<string, unknown>;
  createdAt: number;
}

export type TaskPlanItemStatus = 'pending' | 'running' | 'completed' | 'failed';

export interface TaskPlanItem {
  title: string;
  detail: string;
  status?: TaskPlanItemStatus;
}

export interface TaskPlanDocument {
  request: string;
  goal: string;
  plan: TaskPlanItem[];
}

export interface TaskUpdate {
  name?: string;
  memo?: string;
  content?: string;
  workerId?: number;
  workerName?: string;
  status?: 'queue' | 'running' | 'done' | 'failed' | 'cancelled';
  branch?: string;
}

export interface TaskPromptResponse {
  taskId: number;
  sessionId: string;
  messageId: string;
  partId: string;
  prompt: string;
  rawResponse?: string;
}

// Git 相关类型
export interface BranchInfo {
  name: string;
  isCurrent: boolean;
  isRemote: boolean;
}

export interface TerminalSession {
  id: string;
  workDir: string;
  closed: boolean;
}

export interface TerminalPollResponse {
  output: string;
  cursor: number;
  closed: boolean;
  workDir: string;
}

export interface TaskFilesystemRoot {
  id: string;
  label: string;
  path: string;
}

export interface TaskFilesystemEntry {
  name: string;
  path: string;
  isDir: boolean;
}

export interface WorktreeInfo {
  path: string;
  branch: string;
  head: string;
}

// --- V2 Message Types ---

export interface Memory {
  globalPrompt?: string;
  modelPrompt?: string;
  occupationPrompt?: string;
  projectPrompt?: string;
  workerPrompt?: string;
  projectFilePrompt?: Array<{
    path: string;
    prompt: string;
  }>;
  envPrompt?: string;
  entries?: SessionMemoryEntry[];
  history?: Array<{
    role: string;
    content: string;
    callToolInfo?: string;
  }>;
}

export interface TokenCache {
  read: number;
  write: number;
}

export interface MessageTokens {
  input: number;
  output: number;
  reasoning: number;
  cache: TokenCache;
}

export interface SessionInfo {
  id: string;
  slug: string;
  projectID: string;
  directory: string;
  parentID?: string;
  title: string;
  version: string;
  startSnapshot?: string;
  tokens?: MessageTokens;
  memoryAnalysis?: SessionMemoryAnalysis | null;
  criticalInfo?: SessionCriticalInfo | null;
  time: {
    created: number;
    updated: number;
    compacting?: number;
    archived?: number;
  };
}

export interface SessionMemoryAnalysis {
  keywords: string[];
  summary: string;
  updatedAt: number;
}

export interface SessionCriticalInfo {
  items: CriticalInfoItem[];
}

export interface CriticalInfoItem {
  id: string;
  type: string;
  marker: string;
  message: string;
  createdAt: number;
  asyncTask?: AsyncToolTaskMeta;
}

export interface AsyncToolTaskMeta {
  callId: string;
  toolName: string;
  params?: Record<string, unknown>;
  status: string;
  startedAt: number;
  taskId?: number;
}

export interface SessionPromptResponse {
  sessionId: string;
  messageId: string;
  partId: string;
  prompt: string;
  rawResponse?: string;
}

export interface SessionLoadedSkillInfo {
  name: string;
  description?: string;
  relativePath?: string;
}

export interface SessionToolInfo {
  name: string;
  verbosName?: string;
  description?: string;
  isMcp?: boolean;
  mcpServer?: string;
}

export interface SessionContextComponent {
  key: string;
  label: string;
  bytes: number;
  percent: number;
}

export interface SessionContextResponse {
  sessionId: string;
  workerName: string;
  tools: SessionToolInfo[];
  loadedSkills: SessionLoadedSkillInfo[];
  projectFilePrompts?: Array<{ path: string; prompt: string }>;
  prompts?: Memory;
  components: SessionContextComponent[];
  totalBytes: number;
  contextLimit: number;
  outputLimit: number;
  effectiveContextLimit: number;
}

export interface SessionMemoryEntry {
  id: number;
  sessionID: string;
  sourceMessageID?: string;
  sourcePartID?: string;
  entryKind: string;
  role: string;
  content?: string;
  rawOutput?: string;
  callToolInfo?: string;
  toolCallID?: string;
  toolName?: string;
  toolStatus?: string;
  toolReason?: string;
  toolRequestRawJSON?: string;
  toolInputJSON?: string;
  toolOutput?: string;
  toolError?: string;
  toolTitle?: string;
  toolMetadataJSON?: string;
  synthetic?: boolean;
  compressionLevel?: number;
  sequence: number;
  tokenCount: number;
  created: number;
  updated: number;
}

export interface SessionMemoryCompactionPreview {
  message: string;
  count: number;
  scopePercent: number;
  targetPercent?: number;
  l2ScopePercent?: number;
  levelsExecuted?: number[];
  beforeCount: number;
  afterCount: number;
  compressionRate: number;
  beforePreview: string;
  afterPreview: string;
  summary: string;
}

export interface SessionMemoryEntryInput {
  entryKind: string;
  role: string;
  content?: string;
  rawOutput?: string;
  callToolInfo?: string;
  toolCallID?: string;
  toolName?: string;
  toolStatus?: string;
  toolReason?: string;
  toolRequestRawJSON?: string;
  toolInputJSON?: string;
  toolOutput?: string;
  toolError?: string;
  toolTitle?: string;
  toolMetadataJSON?: string;
  tokenCount?: number;
  synthetic?: boolean;
  sequence?: number;
}

export interface SessionMemoryResponse {
  session: SessionInfo;
  memoryEntries: SessionMemoryEntry[];
}

export interface SessionTransferPayload {
  kind: string;
  version: number;
  exportedAt: number;
  sourceSessionId?: string;
  session?: SessionInfo;
  messages: WithParts[];
  memoryEntries: SessionMemoryEntry[];
}

export interface MessageInfo {
  id: string;
  sessionID: string;
  role: 'user' | 'assistant' | 'system';
  /** user=普通用户消息；system=系统/提醒等注入消息（role 仍为 user 时用于 UI 区分） */
  messageKind?: 'user' | 'system';
  /** 系统消息来源，如 reminder、stall_watchdog */
  messageOrigin?: string;
  parentID?: string;
  mode?: string;
  name?: string;
  occupation?: string;
  worker?: string;
  agent?: string;
  modelID?: string;
  state?: 'loading' | 'reasoning' | 'call-tool' | 'completed';
  memory?: Memory;
  tokens?: MessageTokens;
  time: {
    created: number;
    completed?: number;
  };
  /** 助手消息底部一行状态（实时 WS，可不落库） */
  footerStatus?: { text: string; loading: boolean };
}

export interface PartTime {
  start?: number;
  end?: number;
  created?: number;
  compacted?: number;
}

export interface ToolState {
  status: string;
  input?: any;
  raw?: string;
  title?: string;
  output?: string;
  error?: string;
  metadata?: Record<string, any>;
  time: PartTime;
}

export interface ToolPart {
  tool: string;
  callID: string;
  state: ToolState;
  metadata?: Record<string, any>;
}

export type ResourceGroup = Record<string, string[]>

export interface MessageError {
  name: string;
  message?: string;
  statusCode?: number;
  isRetryable?: boolean;
  responseBody?: string;
}

export interface Part {
  id: string;
  messageID: string;
  sessionID: string;
  type: 'text' | 'reasoning' | 'tool' | 'finish' | 'error' | 'start-step' | 'finish-step' | 'text-delta' | 'reasoning-delta' | 'tool-delta' | 'compaction' | 'memory-organization' | 'patch' | 'file';
  text?: string;
  reasoning?: string;
  reason?: string;
  tool?: ToolPart;
  error?: MessageError;
  files?: string[];
  hash?: string;
  time?: PartTime;
  description?: string;
  metadata?: Record<string, any>;
  /** 用户上传文件（API URL / data URL 等），与 agent types.Part 对齐 */
  url?: string;
  path?: string;
  inputSource?: string;
  mime?: string;
  filename?: string;
}

export interface WithParts {
  info: MessageInfo;
  parts: Part[];
}

// ------------------------

export interface TaskExecution {
  id: number;
  taskId: number;
  workerId?: number;
  workerName: string;
  status: 'running' | 'success' | 'failed';
  command: string;
  workDir: string;
  output: string;
  errorMsg: string;
  agentSessionId?: string;  // AI Agent 会话 ID (用于恢复会话)
  startedAt: string;
  finishedAt?: string;
  duration: number;
  createdAt: string;
}

// LLM 配置类型
export interface LLMConfig {
  id: number;
  name: string;
  type: 'openai' | 'claude' | 'custom';
  apiKey: string;
  model: string;
  baseUrl?: string;
  apiType?: 'chat' | 'response';
  /** HTTP(S) 代理完整 URL，须含 scheme，如 http://127.0.0.1:7890 */
  proxy?: string;
  maxRetries?: number;
  /** 使用 OpenAI 兼容 API 的原生 function calling（tool_calls），而非提示词内 JSON 动作 */
  nativeOpenAIToolCalls?: boolean;
  isDefault?: boolean;
  createdAt: string;
  updatedAt: string;
}

// 职业类型
export interface Occupation {
  id: number;
  code: string;
  name: string;
  description: string;
  prompt: string;
  color: string;
  createdAt: string;
  updatedAt: string;
}

export interface OccupationUpdate {
  name?: string;
  description?: string;
  prompt?: string;
  color?: string;
}

// 模型设置类型
export interface ModelSettings {
  name: string;
  contextLimit: number;
  outputLimit: number;
  budgetTokens?: number | null;
  prompt: string;
  systemPromptPlacement: 'system' | 'instruction' | 'user_input';
  nativeOpenAIToolCalls: boolean;
  topP?: number | null;
  topK?: number | null;
  frequencyPenalty?: number | null;
  enableThinking?: boolean | null;
  reasoningEffort?: 'low' | 'medium' | 'high' | 'xhigh' | 'none' | 'max' | null;
  textVerbosity?: 'low' | 'medium' | 'high' | 'xhigh' | null;
  enableEncryptedReasoning?: boolean | null;
  parallelToolCalls?: boolean | null;
  enablePromptCacheKey?: boolean | null;
  enableSilentToolWatchdog?: boolean | null;
  /** 未设置 / enabled / disabled，对应原生请求 thinking.type */
  thinkingType?: '' | 'enabled' | 'disabled';
}

export interface ModelSettingsCreate {
  name: string;
  contextLimit?: number;
  outputLimit?: number;
  budgetTokens?: number;
  prompt?: string;
  systemPromptPlacement?: 'system' | 'instruction' | 'user_input';
  nativeOpenAIToolCalls?: boolean;
  topP?: number;
  topK?: number;
  frequencyPenalty?: number;
  enableThinking?: boolean;
  reasoningEffort?: 'low' | 'medium' | 'high' | 'xhigh' | 'none' | 'max';
  textVerbosity?: 'low' | 'medium' | 'high' | 'xhigh';
  enableEncryptedReasoning?: boolean;
  parallelToolCalls?: boolean;
  enablePromptCacheKey?: boolean;
  enableSilentToolWatchdog?: boolean;
  thinkingType?: '' | 'enabled' | 'disabled';
}

export interface ModelSettingsUpdate {
  name?: string;
  contextLimit?: number | null;
  outputLimit?: number | null;
  budgetTokens?: number | null;
  prompt?: string;
  systemPromptPlacement?: 'system' | 'instruction' | 'user_input';
  nativeOpenAIToolCalls?: boolean;
  topP?: number | null;
  topK?: number | null;
  frequencyPenalty?: number | null;
  enableThinking?: boolean | null;
  reasoningEffort?: 'low' | 'medium' | 'high' | 'xhigh' | 'none' | 'max' | null;
  textVerbosity?: 'low' | 'medium' | 'high' | 'xhigh' | null;
  enableEncryptedReasoning?: boolean | null;
  parallelToolCalls?: boolean | null;
  enablePromptCacheKey?: boolean | null;
  enableSilentToolWatchdog?: boolean | null;
  thinkingType?: '' | 'enabled' | 'disabled' | null;
}

// 编辑器类型
export interface EditorInfo {
  id: string;
  name: string;
  command: string;
  isAvailable: boolean;
}

export interface LLMConfigCreate {
  name: string;
  type: 'openai' | 'claude' | 'custom';
  apiKey: string;
  model: string;
  baseUrl?: string;
  apiType?: 'chat' | 'response';
  proxy?: string;
  maxRetries?: number;
  nativeOpenAIToolCalls?: boolean;
  isDefault?: boolean;
}

export interface LLMConfigUpdate {
  name?: string;
  type?: 'openai' | 'claude' | 'custom';
  apiKey?: string;
  model?: string;
  baseUrl?: string;
  apiType?: 'chat' | 'response';
  proxy?: string;
  maxRetries?: number;
  nativeOpenAIToolCalls?: boolean;
  isDefault?: boolean;
}

export interface LLMModelsPreviewRequest {
  type: 'openai' | 'claude' | 'custom';
  apiKey: string;
  baseUrl?: string;
  proxy?: string;
}

export interface LLMModelsResponse {
  models: string[];
}

export interface LLMDebugRequest {
  input: string;
  configId?: number;
  model?: string;
  temperature?: number;
  maxTokens?: number;
}

export interface LLMDebugResponse {
  text: string;
}

// 命令执行日志
export interface CommandLog {
  id: number;
  source: string;
  sourceId?: number;
  sourceName?: string;
  command: string;
  args: string;
  workDir: string;
  stdinData?: string;
  stdout?: string;
  stderr?: string;
  exitCode?: number;
  error?: string;
  duration: number;
  status: 'running' | 'success' | 'failed';
  createdAt: string;
  finishedAt?: string;
  fields?: CommandLogField[];
}

export interface CommandLogField {
  key: string;
  label: string;
  value: string;
  tone?: string;
}

export interface CommandLogQuery {
  source?: string;
  sourceId?: number;
  status?: string;
  limit?: number;
  offset?: number;
}

export interface CommandLogStats {
  total: number;
  bySource: Record<string, number>;
  byStatus: Record<string, number>;
}

export interface UsageAnalyticsQuery {
  start?: number;
  end?: number;
  bucket?: 'hour' | 'day';
  taskId?: number;
}

export interface UsageAnalyticsRange {
  start: number;
  end: number;
  bucket: 'hour' | 'day';
}

// iLink 微信账号
export interface WechatAccount {
  id: number;
  botId: string;
  botToken: string;
  baseURL: string;
  ilinkUserId: string;
  status: 'online' | 'offline' | 'error';
  enabled: boolean;
  boundTaskId?: number;
  createdAt: string;
  updatedAt: string;
}

export interface WechatAccountUpdate {
  enabled?: boolean;
  boundTaskId?: number | null;
}

export interface QRCodeResponse {
  qrcode: string;
  qrcode_img_content: string;
}

export interface UsageAnalyticsTask {
  id: number;
  name: string;
  content: string;
  sessionId: string;
}

export interface UsageAnalyticsSummary {
  assistantMessages: number;
  llmCalls: number;
  failedLLMCalls: number;
  toolCalls: number;
  uniqueTools: number;
  inputTokens: number;
  outputTokens: number;
  reasoningTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  cacheHitCount: number;
  cacheHitRate: number;
  avgFirstByteMs: number;
  p95FirstByteMs: number;
  avgTokenSpeed: number;
}

export interface UsageProviderMetric {
  provider: string;
  calls: number;
  failedCalls: number;
  assistantMessages: number;
  inputTokens: number;
  outputTokens: number;
  reasoningTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  cacheHitCount: number;
  avgFirstByteMs: number;
  p95FirstByteMs: number;
  avgTokenSpeed: number;
}

export interface UsageModelMetric {
  provider: string;
  model: string;
  calls: number;
  failedCalls: number;
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens: number;
  avgFirstByteMs: number;
  avgTokenSpeed: number;
}

export interface UsageToolMetric {
  tool: string;
  calls: number;
  completedCalls: number;
  failedCalls: number;
  avgDurationMs: number;
  totalDurationMs: number;
}

export interface UsageTimelinePoint {
  timestamp: number;
  label: string;
  llmCalls: number;
  toolCalls: number;
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens: number;
  avgFirstByteMs: number;
  avgTokenSpeed: number;
}

export interface UsageToolTimelinePoint {
  timestamp: number;
  label: string;
  toolCalls: number;
}

export interface UsageAnalyticsResponse {
  range: UsageAnalyticsRange;
  summary: UsageAnalyticsSummary;
  task?: UsageAnalyticsTask;
  providers: UsageProviderMetric[];
  models: UsageModelMetric[];
  tools: UsageToolMetric[];
  timeline: UsageTimelinePoint[];
  toolTimeline: UsageToolTimelinePoint[];
}

export type TaskListGroupMode = 'none' | 'date' | 'project';

export interface Workspace {
  id: number;
  type: string;
  name: string;
  path: string;
  icon: string;
  color: string;
  groupMode: TaskListGroupMode;
  active: boolean;
  projectIds: number[];      // 工作区关联的项目ID列表
  projects?: Project[];      // 查询时填充的项目列表
  createdAt: string;
  updatedAt: string;
}

export interface WorkspaceResponse extends Workspace {
  pathExists: boolean;
  error?: string;
}

export interface WorkspaceCreate {
  type?: string; // 可选，默认 code
  name: string;
  path?: string; // 可选，如果为空则使用默认路径
  icon?: string;
  color?: string;
  groupMode?: TaskListGroupMode;
  projects?: ProjectCreate[]; // 关联的项目列表
}

export interface WorkspaceUpdate {
  type?: string;
  name?: string;
  path?: string;
  icon?: string;
  color?: string;
  groupMode?: TaskListGroupMode;
  active?: boolean;
}

export interface TestScenario {
  id: string;
  name: string;
  description: string;
}

export interface TestResult {
  taskId: number;
  verifyTaskId: number;
  status: 'passed' | 'failed' | 'partial' | 'error' | 'running';
  mainTaskOutput: string;
  verificationOutput: string;
  error?: string;
  startedAt: string;
  completedAt: string;
}

/** 桌面端「已打开」列表中的单项（顺序与打开先后一致） */
export type OpenUIApplicationKind = 'workspace' | 'project';

export interface OpenUIApplicationItemView {
  kind: OpenUIApplicationKind;
  workspaceId: number;
  workspace?: WorkspaceResponse;
  project?: ProjectResponse;
}

// API 客户端
class ApiClient {
  private async request<T>(endpoint: string, options: RequestConfig = {}): Promise<T> {
    return httpClient.request<T>(endpoint, options);
  }

  // 工作区相关 API
  async getWorkspaces(): Promise<WorkspaceResponse[]> {
    return this.request<WorkspaceResponse[]>('/workspaces');
  }

  async getWorkspace(id: number): Promise<WorkspaceResponse> {
    return this.request<WorkspaceResponse>(`/workspaces/${id}`);
  }

  async createWorkspace(data: WorkspaceCreate): Promise<WorkspaceResponse> {
    return this.request<WorkspaceResponse>('/workspaces', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateWorkspace(id: number, data: WorkspaceUpdate): Promise<WorkspaceResponse> {
    return this.request<WorkspaceResponse>(`/workspaces/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteWorkspace(id: number): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/workspaces/${id}`, {
      method: 'DELETE',
    });
  }

  async setActiveWorkspace(id: number): Promise<WorkspaceResponse> {
    return this.request<WorkspaceResponse>(`/workspaces/${id}/activate`, {
      method: 'POST',
    });
  }

  // 测试相关 API
  async getTestScenarios(): Promise<TestScenario[]> {
    return this.request<TestScenario[]>('/test/scenarios');
  }

  async runTestScenario(workspaceId: number, scenarioId: string): Promise<TestResult> {
    return this.request<TestResult>(`/test/workspaces/${workspaceId}/run`, {
      method: 'POST',
      body: JSON.stringify({ scenarioId }),
    });
  }

  /** 获取当前已打开的工作区/项目（桌面端工作台） */
  async getOpenUIApplicationItems(): Promise<{ items: OpenUIApplicationItemView[]; lastClosedWorkspace?: WorkspaceResponse }> {
    return this.request<{ items: OpenUIApplicationItemView[]; lastClosedWorkspace?: WorkspaceResponse }>('/ui/open');
  }

  async registerOpenWorkspaceInUI(id: number): Promise<{ ok: boolean }> {
    return this.request<{ ok: boolean }>(`/ui/open/workspaces/${id}`, {
      method: 'POST',
    });
  }

  async unregisterOpenWorkspaceFromUI(id: number): Promise<{ ok: boolean }> {
    return this.request<{ ok: boolean }>(`/ui/open/workspaces/${id}`, {
      method: 'DELETE',
    });
  }

  async registerOpenProjectInUI(id: number): Promise<{ ok: boolean }> {
    return this.request<{ ok: boolean }>(`/ui/open/projects/${id}`, {
      method: 'POST',
    });
  }

  async unregisterOpenProjectFromUI(id: number): Promise<{ ok: boolean }> {
    return this.request<{ ok: boolean }>(`/ui/open/projects/${id}`, {
      method: 'DELETE',
    });
  }

  // 项目相关 API
  async getAllProjects(): Promise<ProjectResponse[]> {
    return this.request<ProjectResponse[]>('/projects');
  }

  async getProjects(workspaceId: number): Promise<ProjectResponse[]> {
    return this.request<ProjectResponse[]>(`/workspaces/${workspaceId}/projects`);
  }

  async getResources(projectId: number): Promise<ResourceGroup[]> {
    return this.request<ResourceGroup[]>(`/resources?projectId=${projectId}`);
  }

  async searchResources(projectId: number, query: string): Promise<string[]> {
    return this.request<string[]>(`/resources/search?projectId=${projectId}&query=${encodeURIComponent(query)}`)
  }

  async getProject(id: number): Promise<ProjectResponse> {
    return this.request<ProjectResponse>(`/projects/${id}`);
  }

  // 创建独立项目（不关联工作区）
  async createStandaloneProject(data: ProjectCreate): Promise<Project> {
    return this.request<Project>(`/projects`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  // 创建项目并关联到工作区
  async createProject(workspaceId: number, data: ProjectCreate): Promise<Project> {
    return this.request<Project>(`/workspaces/${workspaceId}/projects`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateProject(id: number, data: Partial<Project>): Promise<Project> {
    return this.request<Project>(`/projects/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteProject(id: number): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/projects/${id}`, {
      method: 'DELETE',
    });
  }

  // 添加已存在的项目到工作区
  async addProjectToWorkspace(workspaceId: number, projectId: number): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/workspaces/${workspaceId}/projects/${projectId}`, {
      method: 'POST',
    });
  }

  // 从工作区移除项目（不删除项目）
  async removeProjectFromWorkspace(workspaceId: number, projectId: number): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/workspaces/${workspaceId}/projects/${projectId}`, {
      method: 'DELETE',
    });
  }

  // 记忆库 / RAG API
  async getMemoryLibraries(options?: { includeTemporary?: boolean; isRag?: boolean }): Promise<MemoryLibrary[]> {
    const params = new URLSearchParams();
    if (options?.includeTemporary) params.set('includeTemporary', 'true');
    if (options?.isRag) params.set('isRag', 'true');
    const query = params.toString();
    return this.request<MemoryLibrary[]>(`/memory-libraries${query ? `?${query}` : ''}`);
  }

  async getRagLibraries(options?: { includeTemporary?: boolean }): Promise<MemoryLibrary[]> {
    return this.getMemoryLibraries({ ...options, isRag: true });
  }

  async getMemoryLibrary(id: number): Promise<MemoryLibrary> {
    return this.request<MemoryLibrary>(`/memory-libraries/${id}`);
  }

  async createMemoryLibrary(data: MemoryLibraryCreate): Promise<MemoryLibrary> {
    return this.request<MemoryLibrary>('/memory-libraries', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async createRagLibrary(data: Omit<MemoryLibraryCreate, 'isRag'>): Promise<MemoryLibrary> {
    return this.createMemoryLibrary({ ...data, isRag: true });
  }

  async updateMemoryLibrary(id: number, data: MemoryLibraryUpdate): Promise<MemoryLibrary> {
    return this.request<MemoryLibrary>(`/memory-libraries/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteMemoryLibrary(id: number): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/memory-libraries/${id}`, {
      method: 'DELETE',
    });
  }

  async promoteMemoryLibrary(id: number, data?: { name?: string }): Promise<MemoryLibrary> {
    return this.request<MemoryLibrary>(`/memory-libraries/${id}/promote`, {
      method: 'POST',
      body: JSON.stringify(data ?? {}),
    });
  }

  // Git API
  async checkGitRepo(projectId: number): Promise<{ isGitRepo: boolean }> {
    return httpClient.get<{ isGitRepo: boolean }>(`/projects/${projectId}/git/check`, {
      skipErrorNotification: true, // 跳过错误通知，检查失败是正常情况
    });
  }

  async initGitRepo(projectId: number, commitMessage?: string): Promise<{ message: string; project: string; path: string }> {
    return this.request<{ message: string; project: string; path: string }>(`/projects/${projectId}/git/init`, {
      method: 'POST',
      body: JSON.stringify({ commitMessage }),
    });
  }

  async getBranches(projectId: number): Promise<BranchInfo[]> {
    return this.request<BranchInfo[]>(`/projects/${projectId}/branches`);
  }

  async getCurrentBranch(projectId: number): Promise<{ branch: string }> {
    return this.request<{ branch: string }>(`/projects/${projectId}/branch/current`);
  }

  async getDefaultBranch(projectId: number): Promise<{ branch: string }> {
    return this.request<{ branch: string }>(`/projects/${projectId}/branch/default`);
  }

  async getWorktrees(projectId: number): Promise<WorktreeInfo[]> {
    return this.request<WorktreeInfo[]>(`/projects/${projectId}/worktrees`);
  }

  async createWorktree(projectId: number, data: { newBranch: string; baseBranch: string }): Promise<{ message: string; path: string; branch: string }> {
    return this.request<{ message: string; path: string; branch: string }>(`/projects/${projectId}/worktrees`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async deleteWorktree(projectId: number, path: string): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/projects/${projectId}/worktrees?path=${encodeURIComponent(path)}`, {
      method: 'DELETE',
    });
  }

  async createTerminalSession(data: { workDir?: string }): Promise<TerminalSession> {
    return this.request<TerminalSession>('/terminals/sessions', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async pollTerminalSession(id: string, cursor: number): Promise<TerminalPollResponse> {
    return this.request<TerminalPollResponse>(`/terminals/sessions/${id}?cursor=${cursor}`, {
      skipErrorNotification: true,
    });
  }

  async writeTerminalSession(id: string, input: string): Promise<{ ok: boolean }> {
    return this.request<{ ok: boolean }>(`/terminals/sessions/${id}/input`, {
      method: 'POST',
      body: JSON.stringify({ input }),
      skipErrorNotification: true,
    });
  }

  async resizeTerminalSession(id: string, data: { cols: number; rows: number }): Promise<{ ok: boolean }> {
    return this.request<{ ok: boolean }>(`/terminals/sessions/${id}/resize`, {
      method: 'POST',
      body: JSON.stringify(data),
      skipErrorNotification: true,
    });
  }

  async closeTerminalSession(id: string): Promise<{ ok: boolean }> {
    return this.request<{ ok: boolean }>(`/terminals/sessions/${id}`, {
      method: 'DELETE',
      skipErrorNotification: true,
    });
  }

  async getTaskFilesystemRoots(taskId: number): Promise<{ roots: TaskFilesystemRoot[] }> {
    return this.request<{ roots: TaskFilesystemRoot[] }>(`/tasks/${taskId}/filesystem/roots`);
  }

  async listTaskFilesystem(taskId: number, root: string, path = ''): Promise<TaskFilesystemEntry[]> {
    const params = new URLSearchParams({ root });
    if (path) params.set('path', path);
    const response = await this.request<{ entries: TaskFilesystemEntry[] }>(
      `/tasks/${taskId}/filesystem/list?${params.toString()}`,
    );
    return response.entries;
  }

  async readTaskFilesystem(taskId: number, root: string, path: string): Promise<{ content: string; binary: boolean }> {
    const params = new URLSearchParams({ root, path });
    return this.request<{ content: string; binary: boolean }>(
      `/tasks/${taskId}/filesystem/read?${params.toString()}`,
    );
  }

  async writeTaskFilesystem(
    taskId: number,
    data: { root: string; path: string; content: string },
  ): Promise<{ ok: boolean }> {
    return this.request<{ ok: boolean }>(`/tasks/${taskId}/filesystem/write`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  // Provider API
  async getProviders(): Promise<Provider[]> {
    return this.request<Provider[]>('/providers');
  }

  async updateProvider(name: string, data: { enabled: boolean }): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/providers/${name}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  // Worker API
  async getWorkers(): Promise<Worker[]> {
    return this.request<Worker[]>('/workers');
  }

  async createWorker(data: WorkerCreate): Promise<Worker> {
    return this.request<Worker>('/workers', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateWorker(id: number, data: WorkerUpdate): Promise<Worker> {
    return this.request<Worker>(`/workers/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteWorker(id: number): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/workers/${id}`, {
      method: 'DELETE',
    });
  }

  async restoreDefaultWorkers(): Promise<{ message: string }> {
    return this.request<{ message: string }>('/workers/restore-defaults', {
      method: 'POST',
    });
  }

  async bulkApplyWorkerConfig(data: WorkerBulkApplyConfigRequest): Promise<{ message: string; updated: number }> {
    return this.request<{ message: string; updated: number }>('/workers/bulk-apply-config', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async getSkillSources(): Promise<SkillSource[]> {
    return this.request<SkillSource[]>('/skill-sources');
  }

  async createSkillSource(data: SkillSourceCreate): Promise<SkillSource> {
    return this.request<SkillSource>('/skill-sources', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateSkillSource(id: number, data: SkillSourceUpdate): Promise<SkillSource> {
    return this.request<SkillSource>(`/skill-sources/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteSkillSource(id: number): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/skill-sources/${id}`, {
      method: 'DELETE',
    });
  }

  async syncSkillSource(id: number): Promise<SkillSource> {
    return this.request<SkillSource>(`/skill-sources/${id}/sync`, {
      method: 'POST',
    });
  }

  async getSkills(installedOnly = false): Promise<SkillCard[]> {
    const query = installedOnly ? '?installedOnly=true' : '';
    return this.request<SkillCard[]>(`/skills${query}`);
  }

  async getMcpServers(): Promise<McpServer[]> {
    return this.request<McpServer[]>('/mcp-servers');
  }

  async createMcpServer(data: McpServerCreate): Promise<McpServer> {
    return this.request<McpServer>('/mcp-servers', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateMcpServer(id: number, data: McpServerUpdate): Promise<McpServer> {
    return this.request<McpServer>(`/mcp-servers/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteMcpServer(id: number): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/mcp-servers/${id}`, {
      method: 'DELETE',
    });
  }

  async reconnectMcpServer(id: number): Promise<McpServer> {
    return this.request<McpServer>(`/mcp-servers/${id}/reconnect`, {
      method: 'POST',
    });
  }

  async getMcpServerTools(id: number): Promise<McpToolInfo[]> {
    return this.request<McpToolInfo[]>(`/mcp-servers/${id}/tools`);
  }

  async getSearchConfigs(): Promise<SearchConfig[]> {
    return this.request<SearchConfig[]>('/search-configs');
  }

  async createSearchConfig(data: SearchConfigCreate): Promise<SearchConfig> {
    return this.request<SearchConfig>('/search-configs', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateSearchConfig(id: number, data: SearchConfigUpdate): Promise<SearchConfig> {
    return this.request<SearchConfig>(`/search-configs/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteSearchConfig(id: number): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/search-configs/${id}`, {
      method: 'DELETE',
    });
  }

  async testSearchConfig(
    id: number,
    data: { query: string; limit?: number; enablePageCrawling?: boolean }
  ): Promise<SearchTestResponse> {
    return this.request<SearchTestResponse>(`/search-configs/${id}/test`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async getEmbeddingConfigs(): Promise<EmbeddingConfig[]> {
    return this.request<EmbeddingConfig[]>('/embedding-configs');
  }

  async createEmbeddingConfig(data: EmbeddingConfigCreate): Promise<EmbeddingConfig> {
    return this.request<EmbeddingConfig>('/embedding-configs', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateEmbeddingConfig(id: number, data: EmbeddingConfigUpdate): Promise<EmbeddingConfig> {
    return this.request<EmbeddingConfig>(`/embedding-configs/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteEmbeddingConfig(id: number): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/embedding-configs/${id}`, {
      method: 'DELETE',
    });
  }

  async testEmbeddingConfig(id: number, data: { sample?: string }): Promise<EmbeddingTestResponse> {
    return this.request<EmbeddingTestResponse>(`/embedding-configs/${id}/test`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async getMemoryLibrarySearchIndexStatus(libraryId: number): Promise<MemoryLibrarySearchIndexStatus> {
    return this.request<MemoryLibrarySearchIndexStatus>(`/memory-libraries/${libraryId}/search-index/status`);
  }

  async rebuildMemoryLibrarySearchIndex(libraryId: number): Promise<{ id: number; status: string }> {
    return this.request<{ id: number; status: string }>(`/memory-libraries/${libraryId}/search-index/rebuild`, {
      method: 'POST',
    });
  }

  async installSkill(sourceId: number, relativePath: string): Promise<{ message: string; path: string }> {
    return this.request<{ message: string; path: string }>('/skills/install', {
      method: 'POST',
      body: JSON.stringify({ sourceId, relativePath }),
    });
  }

  async uninstallSkill(sourceId: number, relativePath: string): Promise<{ message: string }> {
    return this.request<{ message: string }>('/skills/uninstall', {
      method: 'POST',
      body: JSON.stringify({ sourceId, relativePath }),
    });
  }

  async getTools(): Promise<ToolInfo[]> {
    return this.request<ToolInfo[]>('/tools');
  }

  async exportWorkers(ids: number[]): Promise<Blob> {
    const response = await fetch(`${API_BASE_URL}/workers/export`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ ids }),
    });

    if (!response.ok) {
      let data: any = {};
      try {
        data = await response.json();
      } catch {
        data = {};
      }
      throw {
        message: data.error || data.message || `请求失败 (HTTP ${response.status})`,
        status: response.status,
        data,
      };
    }

    return response.blob();
  }

  async importWorkers(file: File): Promise<{ message: string; imported: number; updated: number }> {
    const formData = new FormData();
    formData.append('file', file);
    const response = await fetch(`${API_BASE_URL}/workers/import`, {
      method: 'POST',
      body: formData,
    });

    if (!response.ok) {
      let data: any = {};
      try {
        data = await response.json();
      } catch {
        data = {};
      }
      throw {
        message: data.error || data.message || `请求失败 (HTTP ${response.status})`,
        status: response.status,
        data,
      };
    }

    return response.json();
  }

  // Task API
  async getTasks(workspaceId: number): Promise<Task[]> {
    return this.request<Task[]>(`/workspaces/${workspaceId}/tasks`);
  }

  async getTask(id: number): Promise<Task> {
    return this.request<Task>(`/tasks/${id}`);
  }

  async uploadTaskUserInputFiles(
    taskId: number,
    files: Array<{ file: File; inputSource?: 'paste' | 'picker' | 'drop' }>,
  ): Promise<{
    files: Array<{
      path: string;
      mime: string;
      filename: string;
      inputSource?: string;
      url: string;
    }>;
  }> {
    const formData = new FormData();
    for (const item of files) {
      formData.append('files', item.file, item.file.name);
      if (item.inputSource) {
        formData.append('inputSource', item.inputSource);
      }
    }
    const response = await fetch(`${API_BASE_URL}/tasks/${taskId}/user-input-files`, {
      method: 'POST',
      body: formData,
    });
    if (!response.ok) {
      let data: any = {};
      try {
        data = await response.json();
      } catch {
        data = {};
      }
      throw {
        message: data.error || data.message || `上传失败 (HTTP ${response.status})`,
        status: response.status,
        data,
      };
    }
    return response.json();
  }

  async uploadTempFiles(
    files: Array<{ file: File; inputSource?: 'paste' | 'picker' | 'drop' }>,
  ): Promise<{
    files: Array<{
      path: string;
      mime: string;
      filename: string;
      inputSource?: string;
      url: string;
    }>;
  }> {
    const formData = new FormData();
    for (const item of files) {
      formData.append('files', item.file, item.file.name);
      if (item.inputSource) {
        formData.append('inputSource', item.inputSource);
      }
    }
    const response = await fetch(`${API_BASE_URL}/temp-uploads`, {
      method: 'POST',
      body: formData,
    });
    if (!response.ok) {
      let data: any = {};
      try {
        data = await response.json();
      } catch {
        data = {};
      }
      throw {
        message: data.error || data.message || `上传失败 (HTTP ${response.status})`,
        status: response.status,
        data,
      };
    }
    return response.json();
  }

  async getTaskPrompt(id: number, options?: { skipErrorNotification?: boolean; messageId?: string }): Promise<TaskPromptResponse> {
    const query = options?.messageId ? `?messageId=${encodeURIComponent(options.messageId)}` : '';
    return this.request<TaskPromptResponse>(`/tasks/${id}/prompt${query}`, {
      skipErrorNotification: options?.skipErrorNotification,
    });
  }

  async createTask(workspaceId: number, data: TaskCreate): Promise<Task> {
    return this.request<Task>(`/workspaces/${workspaceId}/tasks`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async reorderTasks(workspaceId: number, taskIds: number[]): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/workspaces/${workspaceId}/tasks/reorder`, {
      method: 'PUT',
      body: JSON.stringify({ taskIds }),
    });
  }

  async runTask(workspaceId: number, data: TaskCreate): Promise<Task> {
    return this.request<Task>(`/workspaces/${workspaceId}/tasks/run`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async getWorkspaceTasks(workspaceId: number): Promise<Task[]> {
    return this.request<Task[]>(`/workspaces/${workspaceId}/tasks`);
  }

  async updateTask(id: number, data: TaskUpdate): Promise<Task> {
    return this.request<Task>(`/tasks/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteTask(id: number): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/tasks/${id}`, {
      method: 'DELETE',
    });
  }

  async restartTask(id: number): Promise<{ message: string }> {
    // 使用新的 restart API
    return this.request<{ message: string }>(`/tasks/${id}/restart`, {
      method: 'POST',
    });
  }

  // 任务消息队列管理
  async getTaskPlan(id: number): Promise<{ plan: TaskPlanDocument | null }> {
    return this.request<{ plan: TaskPlanDocument | null }>(`/tasks/${id}/plan`);
  }

  async getTaskQueue(id: number): Promise<{ queue: TaskMessageQueueItem[]; autoSend: boolean }> {
    return this.request<{ queue: TaskMessageQueueItem[]; autoSend: boolean }>(`/tasks/${id}/queue`);
  }

  async updateTaskQueue(
    id: number,
    queue: TaskMessageQueueItem[],
    options?: { autoSend?: boolean },
  ): Promise<{ queue: TaskMessageQueueItem[]; autoSend: boolean }> {
    return this.request<{ queue: TaskMessageQueueItem[]; autoSend: boolean }>(`/tasks/${id}/queue`, {
      method: 'PUT',
      body: JSON.stringify({ queue, autoSend: options?.autoSend }),
    });
  }

  async sendNextTaskQueueItem(id: number, itemId: string): Promise<{ message: string; queue: TaskMessageQueueItem[] }> {
    return this.request<{ message: string; queue: TaskMessageQueueItem[] }>(`/tasks/${id}/queue/${itemId}/send-next`, {
      method: 'POST',
    });
  }

  // 已删除 sendTaskMessage，统一使用 WebSocket 发送消息
  // 请使用 useGlobalWebSocket().sendMessage(taskId, content)

  // 获取 V2 历史消息（分页）
  async getTaskLogsV2(
    id: number,
    options?: {
      limit?: number;
      beforeMessageId?: string;
    },
  ): Promise<{
    items: WithParts[];
    hasMore: boolean;
    nextBeforeMessageId?: string;
  }> {
    const params = new URLSearchParams();
    if (options?.limit) params.set('limit', String(options.limit));
    if (options?.beforeMessageId) params.set('beforeMessageId', options.beforeMessageId);
    const query = params.toString();
    return this.request<{
      items: WithParts[];
      hasMore: boolean;
      nextBeforeMessageId?: string;
    }>(`/tasks/${id}/logsv2${query ? `?${query}` : ''}`);
  }

  async getSessionLogsV2(
    sessionId: string,
    options?: {
      limit?: number;
      beforeMessageId?: string;
    },
  ): Promise<{
    items: WithParts[];
    hasMore: boolean;
    nextBeforeMessageId?: string;
  }> {
    const params = new URLSearchParams();
    if (options?.limit) params.set('limit', String(options.limit));
    if (options?.beforeMessageId) params.set('beforeMessageId', options.beforeMessageId);
    const query = params.toString();
    return this.request<{
      items: WithParts[];
      hasMore: boolean;
      nextBeforeMessageId?: string;
    }>(`/sessions/${encodeURIComponent(sessionId)}/logsv2${query ? `?${query}` : ''}`);
  }

  async getSessionInfo(sessionId: string): Promise<SessionInfo> {
    return this.request<SessionInfo>(`/sessions/${sessionId}`);
  }

  async getSessionPrompt(sessionId: string, options?: { skipErrorNotification?: boolean; messageId?: string }): Promise<SessionPromptResponse> {
    const query = options?.messageId ? `?messageId=${encodeURIComponent(options.messageId)}` : '';
    return this.request<SessionPromptResponse>(`/sessions/${sessionId}/prompt${query}`, {
      skipErrorNotification: options?.skipErrorNotification,
    });
  }

  async getSessionContext(sessionId: string, workerName?: string, options?: { skipErrorNotification?: boolean }): Promise<SessionContextResponse> {
    const query = workerName ? `?workerName=${encodeURIComponent(workerName)}` : '';
    return this.request<SessionContextResponse>(`/sessions/${sessionId}/context${query}`, {
      skipErrorNotification: options?.skipErrorNotification,
    });
  }

  async removeSessionSkill(sessionId: string, name: string): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/sessions/${sessionId}/skills/remove`, {
      method: 'POST',
      body: JSON.stringify({ name }),
    });
  }

  async getSessionMemory(sessionId: string): Promise<SessionMemoryResponse> {
    return this.request<SessionMemoryResponse>(`/sessions/${sessionId}/memory`);
  }

  async exportSessionTransfer(sessionId: string): Promise<Blob> {
    const response = await fetch(`${API_BASE_URL}/sessions/${sessionId}/export`, {
      method: 'GET',
    });
    if (!response.ok) {
      throw new Error(`导出失败: ${response.status}`);
    }
    return response.blob();
  }

  async importSessionTransfer(
    sessionId: string,
    file: File,
  ): Promise<{
    message: string;
    sessionId: string;
    importedMessages: number;
    importedMemories: number;
    sourceSessionId?: string;
    appliedSessionTitle?: string;
  }> {
    const body = await file.arrayBuffer();
    const response = await fetch(`${API_BASE_URL}/sessions/${sessionId}/import`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/octet-stream',
      },
      body,
    });
    if (!response.ok) {
      const errorData = await response.json().catch(() => ({ error: '导入失败' }));
      throw new Error(errorData.error || `导入失败: ${response.status}`);
    }
    return response.json();
  }

  async createSessionMemoryEntry(sessionId: string, data: SessionMemoryEntryInput): Promise<SessionMemoryEntry> {
    return this.request<SessionMemoryEntry>(`/sessions/${sessionId}/memory/entries`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async compressSessionMemoryEntries(sessionId: string, ids: number[]): Promise<{ message: string; count: number }> {
    return this.request<{ message: string; count: number }>(`/sessions/${sessionId}/memory/entries/compress`, {
      method: 'POST',
      body: JSON.stringify({ ids }),
    });
  }

  async analyzeSessionMemory(sessionId: string): Promise<SessionMemoryAnalysis> {
    return this.request<SessionMemoryAnalysis>(`/sessions/${sessionId}/memory/analysis`, {
      method: 'POST',
    });
  }

  async compactSessionMemory(sessionId: string): Promise<{
    message: string;
    count: number;
    scopePercent: number;
    beforeCount: number;
    afterCount: number;
    compressionRate: number;
    beforePreview: string;
    afterPreview: string;
    summary: string;
  }> {
    return this.request<{
      message: string;
      count: number;
      scopePercent: number;
      beforeCount: number;
      afterCount: number;
      compressionRate: number;
      beforePreview: string;
      afterPreview: string;
      summary: string;
    }>(`/sessions/${sessionId}/memory/compact`, {
      method: 'POST',
    });
  }

  async previewSessionMemoryCompaction(sessionId: string, taskId?: number): Promise<SessionMemoryCompactionPreview> {
    return this.request<SessionMemoryCompactionPreview>(`/sessions/${sessionId}/memory/organization/preview`, {
      method: 'POST',
      params: taskId ? { taskId } : undefined,
    })
  }

  async streamPreviewSessionMemoryCompaction(
    sessionId: string,
    taskId: number | undefined,
    handlers: {
      onMeta?: (meta: {
        count: number;
        scopePercent: number;
        targetPercent?: number;
        l2ScopePercent?: number;
        beforeCount: number;
        beforePreview: string;
      }) => void
      onDelta?: (summary: string) => void
      onDone?: (preview: SessionMemoryCompactionPreview) => void
      onError?: (message: string) => void
    },
  ): Promise<void> {
    const params = new URLSearchParams()
    if (taskId) {
      params.set('taskId', String(taskId))
    }
    const query = params.toString()
    const url = `${API_BASE_URL}/sessions/${sessionId}/memory/organization/preview/stream${query ? `?${query}` : ''}`
    const response = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
    })
    if (!response.ok) {
      let message = `请求失败 (HTTP ${response.status})`
      try {
        const data = await response.json()
        message = data.error || data.message || message
      } catch {
        // ignore parse errors
      }
      handlers.onError?.(message)
      throw new Error(message)
    }
    if (!response.body) {
      const message = '当前浏览器不支持流式响应'
      handlers.onError?.(message)
      throw new Error(message)
    }

    const reader = response.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    let currentEvent = ''
    let currentData = ''

    const dispatchEvent = () => {
      if (!currentEvent || !currentData) {
        currentEvent = ''
        currentData = ''
        return
      }
      try {
        const payload = JSON.parse(currentData)
        switch (currentEvent) {
          case 'meta':
            handlers.onMeta?.(payload)
            break
          case 'delta':
            if (typeof payload.summary === 'string') {
              handlers.onDelta?.(payload.summary)
            }
            break
          case 'done':
            handlers.onDone?.(payload as SessionMemoryCompactionPreview)
            break
          case 'error':
            handlers.onError?.(payload.error || '记忆压缩失败')
            break
          default:
            break
        }
      } catch {
        handlers.onError?.('解析流式响应失败')
      }
      currentEvent = ''
      currentData = ''
    }

    while (true) {
      const { done, value } = await reader.read()
      if (done) {
        break
      }
      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''
      for (const line of lines) {
        if (line.startsWith('event:')) {
          currentEvent = line.slice(6).trim()
          continue
        }
        if (line.startsWith('data:')) {
          currentData = line.slice(5).trim()
          continue
        }
        if (line.trim() === '') {
          dispatchEvent()
        }
      }
    }
    if (currentEvent && currentData) {
      dispatchEvent()
    }
  }

  async applySessionMemoryCompaction(sessionId: string, summary: string): Promise<SessionMemoryCompactionPreview> {
    return this.request<SessionMemoryCompactionPreview>(`/sessions/${sessionId}/memory/organization/apply`, {
      method: 'POST',
      body: JSON.stringify({ summary }),
    })
  }

  async updateSessionMemoryEntry(sessionId: string, entryId: number, data: SessionMemoryEntryInput): Promise<SessionMemoryEntry> {
    return this.request<SessionMemoryEntry>(`/sessions/${sessionId}/memory/entries/${entryId}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    });
  }

  async deleteSessionMemoryEntry(sessionId: string, entryId: number): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/sessions/${sessionId}/memory/entries/${entryId}`, {
      method: 'DELETE',
    });
  }

  async getTaskExecutions(id: number): Promise<TaskExecution[]> {
    return this.request<TaskExecution[]>(`/tasks/${id}/executions`);
  }

  async retryLastUserMessage(taskId: number, messageId: string): Promise<{ message: string; taskId: number; sessionId: string; content: string }> {
    return this.request<{ message: string; taskId: number; sessionId: string; content: string }>(`/tasks/${taskId}/retry-last-user-message`, {
      method: 'POST',
      body: JSON.stringify({ messageId }),
    });
  }


  // ⚠️ 已删除 stopTask, isTaskRunning
  // 统一使用 WebSocket 控制任务，请使用 useGlobalWebSocket() hook：
  //   - stop(taskId) - 停止任务
  //   - restart(taskId) - 重启任务

  // 健康检查
  async healthCheck(): Promise<{ status: string; message: string }> {
    return httpClient.get<{ status: string; message: string }>(`${API_ROOT || ''}/health`, {
      skipErrorNotification: true, // 跳过错误通知
    });
  }

  // Git 操作 API
  async getTaskDiff(
    taskId: number,
    options?: {
      hash?: string;
      toHash?: string;
      basis?: 'default' | 'base' | 'parent';
      gitFrom?: string;
      gitTo?: string;
      atCommit?: string;
      /** false 时附加 patches=0，跳过后端全量扫描会话部件（时间线已带检查点列表） */
      includePatches?: boolean;
    },
  ): Promise<{
    type: 'branch' | 'working' | 'snapshot';
    diff: string;
    files: Array<{
      path: string;
      diff: string;
      additions?: number;
      deletions?: number;
      files?: any[];
    }>;
    patches?: Array<{
      id?: string;
      partId: string;
      hash: string;
      snapshot: string;
      startSnapshot?: string;
      sessionId?: string;
      messageId: string;
      timestamp: number;
      files: string[];
      description?: string;
    }>;
  }> {
    let url = `/tasks/${taskId}/git/diff`;
    const params = new URLSearchParams();
    if (options?.hash) params.set('hash', options.hash);
    if (options?.toHash) params.set('toHash', options.toHash);
    if (options?.basis) params.set('basis', options.basis);
    if (options?.gitFrom) params.set('gitFrom', options.gitFrom);
    if (options?.gitTo) params.set('gitTo', options.gitTo);
    if (options?.atCommit) params.set('atCommit', options.atCommit);
    if (options?.includePatches === false) params.set('patches', '0');
    const query = params.toString();
    if (query) url += `?${query}`;
    return this.request(url);
  }

  async getTaskGitTimeline(taskId: number): Promise<{
    baseBranch: string;
    baseCommitHash: string;
    items: Array<
      | { kind: 'commit'; timestamp: number; commit: { hash: string; unixSec: number; subject: string } }
      | {
          kind: 'snapshot';
          timestamp: number;
          snapshot: {
            id?: string;
            partId: string;
            hash: string;
            snapshot: string;
            startSnapshot?: string;
            sessionId?: string;
            messageId?: string;
            timestamp: number;
            files?: string[];
            description?: string;
          };
        }
    >;
  }> {
    return this.request(`/tasks/${taskId}/git/timeline`);
  }

  async restoreWorktreeRef(taskId: number, ref: string, clean: boolean = true): Promise<{ message: string }> {
    return this.request(`/tasks/${taskId}/git/restore-ref`, {
      method: 'POST',
      body: JSON.stringify({ ref, clean }),
    });
  }

  async gitCommit(taskId: number, message: string): Promise<{ message: string; commit: string }> {
    return this.request<{ message: string; commit: string }>(`/tasks/${taskId}/git/commit`, {
      method: 'POST',
      body: JSON.stringify({ message }),
    });
  }

  async gitMerge(taskId: number, message?: string): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/tasks/${taskId}/git/merge`, {
      method: 'POST',
      body: JSON.stringify({ message }),
    });
  }

  async restoreSnapshot(taskId: number, hash: string, clean: boolean = true): Promise<{ message: string }> {
    return this.request(`/tasks/${taskId}/git/restore`, {
      method: 'POST',
      body: JSON.stringify({ hash, clean }),
    });
  }

  async applyTaskSnapshot(
    taskId: number,
    body: {
      mode: 'undo_step' | 'restore_checkpoint';
      codeSnapshotId?: string;
      partId: string;
    },
  ): Promise<{ message: string }> {
    return this.request(`/tasks/${taskId}/git/snapshot/apply`, {
      method: 'POST',
      body: JSON.stringify(body),
    });
  }

  // LLM 配置 API
  async getLLMConfigs(): Promise<LLMConfig[]> {
    return this.request<LLMConfig[]>('/llm/configs');
  }

  async getLLMConfig(id: number): Promise<LLMConfig> {
    return this.request<LLMConfig>(`/llm/configs/${id}`);
  }

  async createLLMConfig(data: LLMConfigCreate): Promise<LLMConfig> {
    return this.request<LLMConfig>('/llm/configs', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateLLMConfig(id: number, data: LLMConfigUpdate): Promise<LLMConfig> {
    return this.request<LLMConfig>(`/llm/configs/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteLLMConfig(id: number): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/llm/configs/${id}`, {
      method: 'DELETE',
    });
  }

  async getDefaultLLMConfig(): Promise<LLMConfig> {
    return this.request<LLMConfig>('/llm/configs/default');
  }

  async setDefaultLLMConfig(id: number): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/llm/configs/${id}/set-default`, {
      method: 'PUT',
    });
  }

  async getLLMModels(configId: number): Promise<LLMModelsResponse> {
    return this.request<LLMModelsResponse>(`/llm/models?id=${configId}`);
  }

  async previewLLMModels(payload: LLMModelsPreviewRequest): Promise<LLMModelsResponse> {
    return this.request<LLMModelsResponse>('/llm/models/preview', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  }

  async debugLLM(payload: LLMDebugRequest): Promise<LLMDebugResponse> {
    return this.request<LLMDebugResponse>('/llm/debug', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  }

  async generateCommitMessage(diff: string, configId?: number): Promise<{ message: string }> {
    return this.request<{ message: string }>('/llm/generate-commit-message', {
      method: 'POST',
      body: JSON.stringify({ diff, configId }),
    });
  }

  // 命令日志 API
  async getCommandLogs(query?: CommandLogQuery): Promise<{ logs: CommandLog[]; total: number }> {
    const params = new URLSearchParams();
    if (query?.source) params.set('source', query.source);
    if (query?.sourceId) params.set('sourceId', String(query.sourceId));
    if (query?.status) params.set('status', query.status);
    if (query?.limit) params.set('limit', String(query.limit));
    if (query?.offset) params.set('offset', String(query.offset));
    const queryStr = params.toString();
    return this.request<{ logs: CommandLog[]; total: number }>(`/command-logs${queryStr ? `?${queryStr}` : ''}`);
  }

  async getCommandLog(id: number): Promise<CommandLog> {
    return this.request<CommandLog>(`/command-logs/${id}`);
  }

  async getCommandLogStats(): Promise<CommandLogStats> {
    return this.request<CommandLogStats>('/command-logs/stats');
  }

  async clearCommandLogs(days: number = 7): Promise<{ message: string; deleted: number }> {
    return this.request<{ message: string; deleted: number }>(`/command-logs/clear?days=${days}`, {
      method: 'DELETE',
    });
  }

  async getUsageAnalytics(query?: UsageAnalyticsQuery): Promise<UsageAnalyticsResponse> {
    const params = new URLSearchParams();
    if (query?.start) params.set('start', String(query.start));
    if (query?.end) params.set('end', String(query.end));
    if (query?.bucket) params.set('bucket', query.bucket);
    if (query?.taskId) params.set('taskId', String(query.taskId));
    const queryStr = params.toString();
    return this.request<UsageAnalyticsResponse>(`/usage/analytics${queryStr ? `?${queryStr}` : ''}`);
  }

  // ========== 全局配置 API ==========
  async getConfig(key: string, options?: { skipErrorNotification?: boolean }): Promise<GlobalConfig> {
    return this.request<GlobalConfig>(`/config/${key}`, {
      method: 'GET',
      skipErrorNotification: options?.skipErrorNotification,
    });
  }

  async updateConfig(key: string, value: string): Promise<GlobalConfig> {
    return this.request<GlobalConfig>(`/config/${key}`, {
      method: 'PUT',
      body: JSON.stringify({ value }),
    });
  }

  async getCurrentShell(): Promise<CurrentShellResponse> {
    return this.request<CurrentShellResponse>('/config/shell/current', {
      method: 'GET',
      skipErrorNotification: true,
    });
  }
  
  async getKeepProcessAlive(): Promise<{ enabled: boolean }> {
    return this.request<{ enabled: boolean }>('/config/keep-process-alive/status');
  }

  async updateKeepProcessAlive(enabled: boolean): Promise<{ enabled: boolean }> {
    return this.request<{ enabled: boolean }>('/config/keep-process-alive/status', {
      method: 'PUT',
      body: JSON.stringify({ enabled }),
    });
  }

  async getActiveProcesses(): Promise<{ taskIds: number[]; count: number }> {
    return this.request<{ taskIds: number[]; count: number }>('/config/active-processes/list');
  }

  async killProcess(taskId: number): Promise<{ message: string; taskId: number }> {
    return this.request<{ message: string; taskId: number }>('/config/active-processes/kill', {
      method: 'POST',
      body: JSON.stringify({ taskId }),
    });
  }

  // 编辑器相关 API
  async getEditors(): Promise<EditorInfo[]> {
    return this.request<EditorInfo[]>('/editors');
  }

  async openProject(params: { editorId?: string; path?: string }): Promise<{ message: string }> {
    return this.request<{ message: string }>('/editors/open', {
      method: 'POST',
      body: JSON.stringify(params),
    });
  }

  async openInFileManager(params: { path?: string }): Promise<{ message: string }> {
    return this.request<{ message: string }>('/editors/open-folder', {
      method: 'POST',
      body: JSON.stringify(params),
    });
  }

  // ModelSettings API
  async getModelSettings(): Promise<ModelSettings[]> {
    return this.request<ModelSettings[]>('/model-settings');
  }

  async getModelSetting(name: string): Promise<ModelSettings> {
    return this.request<ModelSettings>(`/model-settings/${encodeURIComponent(name)}`);
  }

  async createModelSetting(data: ModelSettingsCreate): Promise<ModelSettings> {
    return this.request<ModelSettings>('/model-settings', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateModelSetting(name: string, data: ModelSettingsUpdate): Promise<ModelSettings> {
    return this.request<ModelSettings>(`/model-settings/${encodeURIComponent(name)}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteModelSetting(name: string): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/model-settings/${encodeURIComponent(name)}`, {
      method: 'DELETE',
    });
  }

  // ========== 提示词管理 API ==========
  // 全局提示词
  async getGlobalPrompt(): Promise<{ prompt: string }> {
    return this.request<{ prompt: string }>('/prompts/global');
  }

  async updateGlobalPrompt(prompt: string): Promise<{ message: string }> {
    return this.request<{ message: string }>('/prompts/global', {
      method: 'PUT',
      body: JSON.stringify({ prompt }),
    });
  }

  // 职业管理
  async getOccupations(): Promise<Occupation[]> {
    return this.request<Occupation[]>('/prompts/occupations');
  }

  async getOccupation(id: number): Promise<Occupation> {
    return this.request<Occupation>(`/prompts/occupations/${id}`);
  }

  async getOccupationByCode(code: string): Promise<Occupation> {
    return this.request<Occupation>(`/prompts/occupations/code/${code}`);
  }

  async updateOccupation(id: number, data: OccupationUpdate): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/prompts/occupations/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  // 项目提示词
  async getProjectPrompt(projectId: number): Promise<{ prompt: string }> {
    return this.request<{ prompt: string }>(`/prompts/projects/${projectId}`);
  }

  async updateProjectPrompt(projectId: number, prompt: string): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/prompts/projects/${projectId}`, {
      method: 'PUT',
      body: JSON.stringify({ prompt }),
    });
  }

  // ========== iLink 微信账号 API ==========
  async getWechatAccounts(): Promise<WechatAccount[]> {
    return this.request<WechatAccount[]>('/ilink/accounts');
  }

  async updateWechatAccount(id: number, data: WechatAccountUpdate): Promise<WechatAccount> {
    return this.request<WechatAccount>(`/ilink/accounts/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteWechatAccount(id: number): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/ilink/accounts/${id}`, {
      method: 'DELETE',
    });
  }

  async startWechatAccount(id: number): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/ilink/accounts/${id}/start`, {
      method: 'POST',
    });
  }

  async stopWechatAccount(id: number): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/ilink/accounts/${id}/stop`, {
      method: 'POST',
    });
  }

  async fetchWechatQRCode(): Promise<QRCodeResponse> {
    return this.request<QRCodeResponse>('/ilink/qrcode');
  }

  async pollWechatQRStatus(qrcode: string): Promise<WechatAccount> {
    return this.request<WechatAccount>(`/ilink/qrcode/status?qrcode=${encodeURIComponent(qrcode)}`);
  }

  async getTasksForBinding(workspaceId?: number): Promise<Task[]> {
    const query = workspaceId ? `?workspaceId=${workspaceId}` : '';
    return this.request<Task[]>(`/ilink/tasks-for-binding${query}`);
  }
}

// 导出单例实例
export const api = new ApiClient();
