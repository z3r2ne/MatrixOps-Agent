import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Activity, BarChart3, Clock3, DatabaseZap, RefreshCw, Wrench } from 'lucide-react';
import {
  api,
  type Task,
  type UsageAnalyticsQuery,
  type UsageAnalyticsResponse,
  type UsageProviderMetric,
  type UsageTimelinePoint,
  type UsageToolMetric,
} from '@/lib/api';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { cn } from '@/lib/utils';
import { toast } from 'sonner';
import { useElectronUsageAnalyticsFilter } from '@/components/electron/electron-main-contexts';

type RangePreset = '24h' | '7d' | '30d' | 'custom';
type FilterMode = 'time' | 'task';

type ChartSeries = {
  key: string;
  label: string;
  colorClassName: string;
  colorValue: string;
  formatter?: (value: number) => string;
};

const RANGE_PRESETS: Array<{ value: RangePreset; label: string }> = [
  { value: '24h', label: '最近 24 小时' },
  { value: '7d', label: '最近 7 天' },
  { value: '30d', label: '最近 30 天' },
  { value: 'custom', label: '自定义' },
];

const TIMELINE_SERIES: ChartSeries[] = [
  { key: 'outputTokens', label: '输出 Tokens', colorClassName: 'text-foreground', colorValue: 'hsl(var(--foreground))', formatter: formatCompactNumber },
  { key: 'cacheReadTokens', label: '缓存命中 Tokens', colorClassName: 'text-muted-foreground', colorValue: 'hsl(var(--muted-foreground))', formatter: formatCompactNumber },
];

const LATENCY_SERIES: ChartSeries[] = [
  { key: 'avgFirstByteMs', label: '首字延时 (ms)', colorClassName: 'text-foreground', colorValue: 'hsl(var(--foreground))', formatter: formatMs },
  { key: 'avgTokenSpeed', label: '平均速度 (tok/s)', colorClassName: 'text-muted-foreground', colorValue: 'hsl(var(--muted-foreground))', formatter: formatRate },
];

export function UsageAnalyticsPage() {
  const electronUsageFilter = useElectronUsageAnalyticsFilter();
  const [searchParams] = useSearchParams();
  const initialTaskFilterMode = typeof electronUsageFilter?.taskId === 'number' && electronUsageFilter.taskId > 0
    ? true
    : searchParams.get('filter') === 'task';
  const initialTaskId = typeof electronUsageFilter?.taskId === 'number' && electronUsageFilter.taskId > 0
    ? String(electronUsageFilter.taskId)
    : searchParams.get('taskId') || '';

  const [filterMode, setFilterMode] = useState<FilterMode>(initialTaskFilterMode ? 'task' : 'time');
  const [preset, setPreset] = useState<RangePreset>('7d');
  const [bucket, setBucket] = useState<'hour' | 'day'>('day');
  const [taskIdInput, setTaskIdInput] = useState(initialTaskId);
  const [startInput, setStartInput] = useState(() => toDateTimeLocal(Date.now() - 7 * 24 * 60 * 60 * 1000));
  const [endInput, setEndInput] = useState(() => toDateTimeLocal(Date.now()));
  const [data, setData] = useState<UsageAnalyticsResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const hasLoadedInitiallyRef = useRef(false);
  const isTaskFilterActive = filterMode === 'task';

  const applyPreset = useCallback((value: RangePreset) => {
    const end = Date.now();
    let start = end - 7 * 24 * 60 * 60 * 1000;
    let nextBucket: 'hour' | 'day' = 'day';

    if (value === '24h') {
      start = end - 24 * 60 * 60 * 1000;
      nextBucket = 'hour';
    } else if (value === '30d') {
      start = end - 30 * 24 * 60 * 60 * 1000;
      nextBucket = 'day';
    } else if (value === '7d') {
      start = end - 7 * 24 * 60 * 60 * 1000;
      nextBucket = 'day';
    }

    setPreset(value);
    if (value !== 'custom') {
      setStartInput(toDateTimeLocal(start));
      setEndInput(toDateTimeLocal(end));
      setBucket(nextBucket);
    }
  }, []);

  const loadAnalytics = useCallback(async () => {
    let taskId = 0;

    if (isTaskFilterActive) {
      taskId = Number(taskIdInput.trim());
      if (!Number.isInteger(taskId) || taskId <= 0) {
        toast.error('Task ID 无效');
        return;
      }
    } else {
      const start = fromDateTimeLocal(startInput);
      const end = fromDateTimeLocal(endInput);
      if (!start || !end || start >= end) {
        toast.error('时间范围无效');
        return;
      }
    }

    setIsLoading(true);
    try {
      let query: UsageAnalyticsQuery;

      if (isTaskFilterActive) {
        const task = await api.getTask(taskId);
        query = {
          ...getTaskAnalyticsRange(task),
          bucket,
          taskId,
        };
      } else {
        query = {
          start: fromDateTimeLocal(startInput),
          end: fromDateTimeLocal(endInput),
          bucket,
        };
      }

      const result = await api.getUsageAnalytics(query);
      setData(result);
    } catch (error) {
      console.error('Failed to load usage analytics:', error);
      toast.error('加载统计数据失败');
    } finally {
      setIsLoading(false);
    }
  }, [bucket, endInput, isTaskFilterActive, startInput, taskIdInput]);

  useEffect(() => {
    if (typeof electronUsageFilter?.taskId === 'number' && electronUsageFilter.taskId > 0) {
      setFilterMode('task');
      setTaskIdInput(String(electronUsageFilter.taskId));
      return;
    }
    const nextTaskFilterMode = searchParams.get('filter') === 'task';
    const nextTaskId = searchParams.get('taskId') || '';
    setFilterMode(nextTaskFilterMode ? 'task' : 'time');
    setTaskIdInput(nextTaskId);
  }, [electronUsageFilter?.taskId, searchParams]);

  useEffect(() => {
    if (hasLoadedInitiallyRef.current) return;
    hasLoadedInitiallyRef.current = true;
    void loadAnalytics();
  }, [loadAnalytics]);

  const providerBars = useMemo(() => {
    if (!data) return [];
    return data.providers.slice(0, 8).map((item) => ({
      label: item.provider,
      value: item.calls,
      detail: `${formatMs(item.avgFirstByteMs)} · ${formatRate(item.avgTokenSpeed)}`,
    }));
  }, [data]);

  const toolBars = useMemo(() => {
    if (!data) return [];
    return data.tools.slice(0, 10).map((item) => ({
      label: item.tool,
      value: item.calls,
      detail: `${formatMs(item.avgDurationMs)} · ${item.failedCalls} 失败`,
    }));
  }, [data]);

  const topProviders = useMemo(() => data?.providers.slice(0, 6) ?? [], [data]);
  const topTools = useMemo(() => data?.tools.slice(0, 10) ?? [], [data]);
  const topModels = useMemo(() => data?.models.slice(0, 8) ?? [], [data]);

  return (
    <div className="flex-1 overflow-y-auto p-8">
      <div className="mx-auto flex max-w-7xl flex-col gap-6">
        <div className="flex flex-wrap items-start justify-end gap-3">
          <Button type="button" variant="outline" size="sm" onClick={() => void loadAnalytics()} disabled={isLoading}>
            <RefreshCw className={cn('mr-2 h-4 w-4', isLoading && 'animate-spin')} />
            刷新
          </Button>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>筛选条件</CardTitle>
            <CardDescription>支持按时间或按任务筛选，二选一；聚合粒度可单独调整。</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <Tabs
              value={filterMode}
              onValueChange={(value) => setFilterMode(value as FilterMode)}
              className="flex flex-col gap-4"
            >
              <TabsList className="grid w-full max-w-sm grid-cols-2">
                <TabsTrigger value="time">按时间筛选</TabsTrigger>
                <TabsTrigger value="task">按 Task 筛选</TabsTrigger>
              </TabsList>
            </Tabs>

            <div className="grid gap-3 md:grid-cols-5">
              {filterMode === 'time' ? (
                <>
                  <div className="flex flex-col gap-2">
                    <span className="text-sm font-medium">时间范围</span>
                    <Select
                      value={preset}
                      onValueChange={(value: RangePreset) => {
                        if (value === 'custom') {
                          setPreset('custom');
                          return;
                        }
                        applyPreset(value);
                      }}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder="选择范围" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectGroup>
                          {RANGE_PRESETS.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {option.label}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </div>

                  <div className="flex flex-col gap-2">
                    <span className="text-sm font-medium">开始时间</span>
                    <Input
                      type="datetime-local"
                      value={startInput}
                      onChange={(event) => {
                        setPreset('custom');
                        setStartInput(event.target.value);
                      }}
                    />
                  </div>

                  <div className="flex flex-col gap-2">
                    <span className="text-sm font-medium">结束时间</span>
                    <Input
                      type="datetime-local"
                      value={endInput}
                      onChange={(event) => {
                        setPreset('custom');
                        setEndInput(event.target.value);
                      }}
                    />
                  </div>
                </>
              ) : (
                <div className="flex flex-col gap-2 md:col-span-3">
                  <span className="text-sm font-medium">Task ID</span>
                  <Input
                    type="number"
                    min="1"
                    placeholder="输入 Task ID"
                    value={taskIdInput}
                    onChange={(event) => setTaskIdInput(event.target.value)}
                  />
                  <span className="text-xs text-muted-foreground">
                    按任务筛选时，统计时间范围会自动跟随该任务的创建和更新时间。
                  </span>
                </div>
              )}

              <div className="flex flex-col gap-2">
                <span className="text-sm font-medium">聚合粒度</span>
                <Select value={bucket} onValueChange={(value: 'hour' | 'day') => setBucket(value)}>
                  <SelectTrigger>
                    <SelectValue placeholder="选择粒度" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="hour">按小时</SelectItem>
                      <SelectItem value="day">按天</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Button type="button" size="sm" onClick={() => void loadAnalytics()} disabled={isLoading}>
                应用筛选
              </Button>
              {filterMode === 'task' && taskIdInput.trim() && (
                <Button type="button" size="sm" variant="outline" onClick={() => setTaskIdInput('')}>
                  清除 Task
                </Button>
              )}
              {data && (
                <Badge variant="secondary">
                  {formatRangeLabel(data.range.start, data.range.end)} · {data.range.bucket === 'hour' ? '按小时' : '按天'}
                </Badge>
              )}
              {data?.task && (
                <Badge variant="outline">
                  Task #{data.task.id} {data.task.name || truncateText(data.task.content, 20)}
                </Badge>
              )}
            </div>
          </CardContent>
        </Card>

        {isLoading || !data ? (
          <AnalyticsSkeleton />
        ) : (
          <>
            <div className="grid gap-3 md:grid-cols-3 xl:grid-cols-5">
              <MetricCard
                icon={<Activity className="h-4 w-4" />}
                title="LLM 调用"
                value={formatCompactNumber(data.summary.llmCalls)}
                description={`${data.summary.failedLLMCalls} 次失败`}
              />
              <MetricCard
                icon={<Clock3 className="h-4 w-4" />}
                title="Provider 首字延时"
                value={formatMs(data.summary.avgFirstByteMs)}
                description={`P95 ${formatMs(data.summary.p95FirstByteMs)}`}
              />
              <MetricCard
                icon={<DatabaseZap className="h-4 w-4" />}
                title="缓存命中"
                value={formatCompactNumber(data.summary.cacheHitCount)}
                description={`${formatPercent(data.summary.cacheHitRate)} · ${formatCompactNumber(data.summary.cacheReadTokens)} tokens`}
              />
              <MetricCard
                icon={<BarChart3 className="h-4 w-4" />}
                title="Token 速度"
                value={formatRate(data.summary.avgTokenSpeed)}
                description="平均输出速度"
              />
              <MetricCard
                icon={<Wrench className="h-4 w-4" />}
                title="工具调用"
                value={formatCompactNumber(data.summary.toolCalls)}
                description={`${data.summary.uniqueTools} 个工具`}
              />
            </div>

            <div className="grid gap-4 xl:grid-cols-[1.6fr_1fr]">
              <Card>
                <CardHeader>
                  <CardTitle>Token 与缓存趋势</CardTitle>
                  <CardDescription>展示输出 tokens 与缓存命中 tokens 的时间走势。</CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-4">
                  <SimpleLineChart data={data.timeline} series={TIMELINE_SERIES} />
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>延时与速度趋势</CardTitle>
                  <CardDescription>展示首字返回延时和平均 token 速度。</CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-4">
                  <SimpleLineChart data={data.timeline} series={LATENCY_SERIES} />
                </CardContent>
              </Card>
            </div>

            <div className="grid gap-4 xl:grid-cols-2">
              <Card>
                <CardHeader>
                  <CardTitle>Provider 调用排行榜</CardTitle>
                  <CardDescription>按调用量排序，同时附带延时和速度摘要。</CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-4">
                  <HorizontalBarList items={providerBars} />
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>工具调用排行榜</CardTitle>
                  <CardDescription>按调用量排序，同时展示平均耗时和失败数。</CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-4">
                  <HorizontalBarList items={toolBars} />
                </CardContent>
              </Card>
            </div>

            <div className="grid gap-4 xl:grid-cols-[1.2fr_1fr]">
              <Card>
                <CardHeader>
                  <CardTitle>Provider 维度</CardTitle>
                  <CardDescription>可直接查看首字延时、token 速度、缓存和失败情况。</CardDescription>
                </CardHeader>
                <CardContent>
                  <ProviderGrid providers={topProviders} />
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Token 总览</CardTitle>
                  <CardDescription>输入、输出、推理与缓存的总体分布。</CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-4">
                  <StackedBars
                    items={[
                      { label: '输入', value: data.summary.inputTokens },
                      { label: '输出', value: data.summary.outputTokens },
                      { label: '推理', value: data.summary.reasoningTokens },
                      { label: '缓存读', value: data.summary.cacheReadTokens },
                    ]}
                  />
                </CardContent>
              </Card>
            </div>

            <div className="grid gap-4 xl:grid-cols-2">
              <Card>
                <CardHeader>
                  <CardTitle>工具明细</CardTitle>
                  <CardDescription>查看高频工具的调用次数、失败次数和平均耗时。</CardDescription>
                </CardHeader>
                <CardContent>
                  <ScrollArea className="h-[360px]">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>工具</TableHead>
                          <TableHead>调用</TableHead>
                          <TableHead>失败</TableHead>
                          <TableHead>平均耗时</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {topTools.map((tool) => (
                          <TableRow key={tool.tool}>
                            <TableCell className="font-medium">{tool.tool}</TableCell>
                            <TableCell>{formatCompactNumber(tool.calls)}</TableCell>
                            <TableCell>{formatCompactNumber(tool.failedCalls)}</TableCell>
                            <TableCell>{formatMs(tool.avgDurationMs)}</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </ScrollArea>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>模型明细</CardTitle>
                  <CardDescription>按输出 token 排序，便于比较不同模型表现。</CardDescription>
                </CardHeader>
                <CardContent>
                  <ScrollArea className="h-[360px]">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Provider / 模型</TableHead>
                          <TableHead>输出 Tokens</TableHead>
                          <TableHead>首字延时</TableHead>
                          <TableHead>速度</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {topModels.map((model) => (
                          <TableRow key={`${model.provider}-${model.model}`}>
                            <TableCell className="font-medium">{model.provider} / {model.model}</TableCell>
                            <TableCell>{formatCompactNumber(model.outputTokens)}</TableCell>
                            <TableCell>{formatMs(model.avgFirstByteMs)}</TableCell>
                            <TableCell>{formatRate(model.avgTokenSpeed)}</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </ScrollArea>
                </CardContent>
              </Card>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

function MetricCard({
  icon,
  title,
  value,
  description,
}: {
  icon: React.ReactNode;
  title: string;
  value: string;
  description: string;
}) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between gap-2 pb-2">
        <div className="flex flex-col gap-1">
          <CardDescription className="text-[11px]">{title}</CardDescription>
          <CardTitle className="text-xl">{value}</CardTitle>
        </div>
        <div className="text-muted-foreground">{icon}</div>
      </CardHeader>
      <CardContent className="pt-0">
        <p className="text-xs text-muted-foreground">{description}</p>
      </CardContent>
    </Card>
  );
}

function ProviderGrid({ providers }: { providers: UsageProviderMetric[] }) {
  return (
    <div className="grid gap-3 md:grid-cols-2">
      {providers.map((provider) => (
        <div key={provider.provider} className="flex flex-col gap-3 border border-border/60 p-4">
          <div className="flex items-center justify-between gap-3">
            <div className="min-w-0">
              <p className="truncate font-medium">{provider.provider}</p>
              <p className="text-sm text-muted-foreground">
                {provider.calls} 次调用 · {provider.failedCalls} 次失败
              </p>
            </div>
            <Badge variant="outline">{formatRate(provider.avgTokenSpeed)}</Badge>
          </div>
          <div className="grid grid-cols-2 gap-3 text-sm">
            <div className="flex flex-col gap-1">
              <span className="text-muted-foreground">首字延时</span>
              <span>{formatMs(provider.avgFirstByteMs)}</span>
            </div>
            <div className="flex flex-col gap-1">
              <span className="text-muted-foreground">P95</span>
              <span>{formatMs(provider.p95FirstByteMs)}</span>
            </div>
            <div className="flex flex-col gap-1">
              <span className="text-muted-foreground">缓存命中</span>
              <span>{formatCompactNumber(provider.cacheHitCount)}</span>
            </div>
            <div className="flex flex-col gap-1">
              <span className="text-muted-foreground">输出 Tokens</span>
              <span>{formatCompactNumber(provider.outputTokens)}</span>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

function SimpleLineChart({
  data,
  series,
}: {
  data: UsageTimelinePoint[];
  series: ChartSeries[];
}) {
  const width = 760;
  const height = 260;
  const padding = 28;
  const chartWidth = width - padding * 2;
  const chartHeight = height - padding * 2;
  const maxValue = Math.max(
    1,
    ...data.flatMap((item) => series.map((entry) => Number(item[entry.key as keyof UsageTimelinePoint] ?? 0)))
  );

  const buildPoints = (key: string) => {
    if (data.length === 1) {
      const x = padding + chartWidth / 2;
      const y = padding + chartHeight - ((Number(data[0][key as keyof UsageTimelinePoint] ?? 0) / maxValue) * chartHeight);
      return `${x},${y}`;
    }
    return data
      .map((item, index) => {
        const x = padding + (index / Math.max(1, data.length - 1)) * chartWidth;
        const value = Number(item[key as keyof UsageTimelinePoint] ?? 0);
        const y = padding + chartHeight - (value / maxValue) * chartHeight;
        return `${x},${y}`;
      })
      .join(' ');
  };

  const axisLabels = data.filter((_, index) => index === 0 || index === data.length - 1 || index % Math.ceil(data.length / 4 || 1) === 0);

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-3">
        {series.map((item) => (
          <div key={item.key} className="flex items-center gap-2 text-sm text-muted-foreground">
            <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: item.colorValue }} />
            <span>{item.label}</span>
          </div>
        ))}
      </div>
      <svg viewBox={`0 0 ${width} ${height}`} className="w-full">
        {[0, 0.25, 0.5, 0.75, 1].map((step) => {
          const y = padding + chartHeight - chartHeight * step;
          return (
            <g key={step}>
              <line x1={padding} x2={width - padding} y1={y} y2={y} stroke="hsl(var(--border))" strokeDasharray="4 4" />
              <text x={8} y={y + 4} className="fill-muted-foreground text-[10px]">
                {formatCompactNumber(Math.round(maxValue * step))}
              </text>
            </g>
          );
        })}
        {series.map((entry) => (
          <polyline
            key={entry.key}
            fill="none"
            stroke={entry.colorValue}
            strokeWidth="2.5"
            points={buildPoints(entry.key)}
          />
        ))}
        {axisLabels.map((item) => {
          const index = data.findIndex((entry) => entry.timestamp === item.timestamp);
          const x = padding + (index / Math.max(1, data.length - 1)) * chartWidth;
          return (
            <text key={item.timestamp} x={x} y={height - 6} textAnchor="middle" className="fill-muted-foreground text-[10px]">
              {item.label}
            </text>
          );
        })}
      </svg>
    </div>
  );
}

function HorizontalBarList({
  items,
}: {
  items: Array<{ label: string; value: number; detail: string }>;
}) {
  const maxValue = Math.max(1, ...items.map((item) => item.value));

  return (
    <div className="flex flex-col gap-3">
      {items.map((item) => (
        <div key={item.label} className="flex flex-col gap-2">
          <div className="flex items-center justify-between gap-3 text-sm">
            <div className="min-w-0">
              <p className="truncate font-medium">{item.label}</p>
              <p className="truncate text-muted-foreground">{item.detail}</p>
            </div>
            <span className="shrink-0 text-muted-foreground">{formatCompactNumber(item.value)}</span>
          </div>
          <div className="h-2 bg-muted">
            <div className="h-2 bg-foreground" style={{ width: `${(item.value / maxValue) * 100}%` }} />
          </div>
        </div>
      ))}
    </div>
  );
}

function StackedBars({
  items,
}: {
  items: Array<{ label: string; value: number }>;
}) {
  const total = items.reduce((sum, item) => sum + item.value, 0);

  return (
    <div className="flex flex-col gap-3">
      {items.map((item) => {
        const width = total > 0 ? (item.value / total) * 100 : 0;
        return (
          <div key={item.label} className="flex flex-col gap-2">
            <div className="flex items-center justify-between gap-2 text-sm">
              <span>{item.label}</span>
              <span className="text-muted-foreground">{formatCompactNumber(item.value)}</span>
            </div>
            <div className="h-2 bg-muted">
              <div className="h-2 bg-foreground" style={{ width: `${width}%` }} />
            </div>
          </div>
        );
      })}
    </div>
  );
}

function AnalyticsSkeleton() {
  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        {Array.from({ length: 4 }).map((_, index) => (
          <Skeleton key={index} className="h-32" />
        ))}
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <Skeleton className="h-[360px]" />
        <Skeleton className="h-[360px]" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <Skeleton className="h-[320px]" />
        <Skeleton className="h-[320px]" />
      </div>
    </div>
  );
}

function toDateTimeLocal(timestamp: number) {
  const date = new Date(timestamp);
  const offset = date.getTimezoneOffset();
  const local = new Date(date.getTime() - offset * 60 * 1000);
  return local.toISOString().slice(0, 16);
}

function fromDateTimeLocal(value: string) {
  const timestamp = new Date(value).getTime();
  return Number.isNaN(timestamp) ? 0 : timestamp;
}

function getTaskAnalyticsRange(task: Task) {
  const createdAt = Date.parse(task.createdAt);
  const updatedAt = Date.parse(task.updatedAt);
  const paddingMs = 5 * 60 * 1000;
  const minimumRangeMs = 60 * 60 * 1000;

  if (!Number.isFinite(createdAt)) {
    throw new Error('Invalid task createdAt');
  }

  return {
    start: Math.max(0, createdAt - paddingMs),
    end: Math.max(
      Number.isFinite(updatedAt) ? updatedAt + paddingMs : Date.now(),
      createdAt + minimumRangeMs,
    ),
  };
}

function formatMs(value: number): string {
  if (!Number.isFinite(value)) return '—';
  return `${Math.round(value)} ms`;
}

function formatRate(value: number): string {
  if (!Number.isFinite(value)) return '—';
  return `${new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 1 }).format(value)} tok/s`;
}

function formatPercent(value: number): string {
  if (!Number.isFinite(value)) return '—';
  return new Intl.NumberFormat('zh-CN', { style: 'percent', maximumFractionDigits: 1 }).format(value);
}

function formatCompactNumber(value: number) {
  if (!Number.isFinite(value)) return '0';
  return new Intl.NumberFormat('zh-CN', { notation: 'compact', maximumFractionDigits: 1 }).format(value);
}

function formatRangeLabel(start: number, end: number) {
  const formatter = new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
  return `${formatter.format(start)} - ${formatter.format(end)}`;
}

function truncateText(value: string, maxLength: number) {
  if (value.length <= maxLength) return value;
  return `${value.slice(0, maxLength)}…`;
}
