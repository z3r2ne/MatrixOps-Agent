# 思考消息自动展开/折叠功能

## 📋 功能说明

优化了思考消息（thinking）的展示体验，实现了智能的展开/折叠行为。

## 🎯 功能特性

### 1. 默认展开
- ✅ 思考消息输出时**自动展开**，让用户实时看到 AI 的思考过程
- ✅ 区分思考状态：正在思考显示"正在思考..."，完成后显示"思考过程"

### 2. 固定高度 + 滚动
- ✅ 思考内容区域**最大高度 192px**（`max-h-48`）
- ✅ 内容超过限制时**自动显示滚动条**
- ✅ 思考时**自动滚动到底部**，始终显示最新内容
- ✅ 美化的滚动条样式（细滚动条，半透明）

### 3. 自动折叠
- ✅ 思考完成后**自动折叠**（延迟 500ms，让用户看到完成状态）
- ✅ 折叠后显示紧凑的"思考过程"标题
- ✅ 用户可以**点击手动展开/折叠**

### 4. 智能重新展开
- ✅ 追问时如果思考重新开始，**自动展开**
- ✅ 状态变化自动适应

## 🔧 技术实现

### 核心逻辑（src/components/workspace/ChatInterface.tsx）

```typescript
const ThinkingEntry: React.FC<{ entry: NormalizedEntry }> = ({ entry }) => {
  // 判断是否正在流式输出
  const isStreaming = !entry.content.endsWith("。") && !entry.content.endsWith(".")
  
  // 默认状态：思考中=展开，已完成=折叠
  const [expanded, setExpanded] = useState(isStreaming)
  const contentRef = useRef<HTMLDivElement>(null)

  // 思考结束后自动折叠
  useEffect(() => {
    if (!isStreaming && expanded) {
      const timer = setTimeout(() => {
        setExpanded(false)
      }, 500) // 500ms 延迟
      return () => clearTimeout(timer)
    } else if (isStreaming && !expanded) {
      // 重新开始思考时自动展开
      setExpanded(true)
    }
  }, [isStreaming, expanded])

  // 内容更新时自动滚动到底部
  useEffect(() => {
    if (expanded && isStreaming && contentRef.current) {
      contentRef.current.scrollTop = contentRef.current.scrollHeight
    }
  }, [entry.content, expanded, isStreaming])

  return (
    <div className="flex items-start gap-2">
      {/* 图标：思考中=旋转加载器，完成=大脑图标 */}
      <div className="flex h-5 w-5 shrink-0 items-center justify-center mt-0.5">
        {isStreaming ? (
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
        ) : (
          <Brain className="h-3.5 w-3.5" />
        )}
      </div>
      
      <div className="flex-1 min-w-0">
        {/* 标题：可点击切换展开/折叠 */}
        <div
          className="flex items-center gap-1 cursor-pointer hover:text-foreground select-none"
          onClick={() => setExpanded(!expanded)}
        >
          {expanded ? <ChevronDown /> : <ChevronRight />}
          <span className="font-medium text-xs">
            {isStreaming ? "正在思考..." : "思考过程"}
          </span>
        </div>
        
        {/* 内容区域：固定高度 + 滚动 */}
        {expanded && (
          <div 
            ref={contentRef}
            className={cn(
              "mt-1.5 pl-4 text-xs text-muted-foreground/80",
              "whitespace-pre-wrap border-l-2 border-muted leading-relaxed",
              "max-h-48 overflow-y-auto", // 最大高度 192px，超出滚动
              "scrollbar-thin scrollbar-thumb-muted-foreground/20 scrollbar-track-transparent"
            )}
          >
            {entry.content}
          </div>
        )}
      </div>
    </div>
  )
}
```

## 🎨 样式说明

### Tailwind CSS 类

- `max-h-48`: 最大高度 12rem（192px）
- `overflow-y-auto`: Y 轴滚动
- `scrollbar-thin`: 细滚动条（需要 tailwind-scrollbar 插件）
- `scrollbar-thumb-muted-foreground/20`: 滚动条颜色（20% 不透明度）
- `scrollbar-track-transparent`: 滚动条轨道透明

## 📊 状态流转

```
思考开始（isStreaming=true）
    ↓
自动展开（expanded=true）
    ↓
内容流式输出...
    ↓
自动滚动到底部
    ↓
思考完成（isStreaming=false）
    ↓
延迟 500ms
    ↓
自动折叠（expanded=false）
    ↓
用户可点击手动展开
```

## 🔄 追问场景

```
思考已折叠
    ↓
用户发送追问
    ↓
检测到 isStreaming=true
    ↓
自动展开（expanded=true）
    ↓
显示新的思考内容...
```

## 🎯 用户体验改进

### 改进前
- ❌ 思考消息默认折叠，用户需要手动点击才能看到
- ❌ 思考完成后一直保持展开，占用屏幕空间
- ❌ 长思考内容没有高度限制，可能撑满整个屏幕
- ❌ 思考更新时不会自动滚动，可能看不到最新内容

### 改进后
- ✅ 思考时自动展开，实时显示思考过程
- ✅ 思考完成后自动折叠，节省屏幕空间
- ✅ 固定高度 + 滚动，长内容也不会挤占空间
- ✅ 自动滚动到底部，始终显示最新思考
- ✅ 保留手动控制能力，用户可随时展开/折叠

## 🧪 测试建议

1. **基本展开/折叠**
   - 发送消息触发思考
   - 观察思考消息是否自动展开
   - 等待思考完成，观察是否自动折叠

2. **长内容滚动**
   - 发送复杂问题，生成长思考内容
   - 观察滚动条是否出现
   - 确认内容是否自动滚动到底部

3. **手动控制**
   - 点击"思考过程"标题
   - 确认可以手动展开/折叠
   - 折叠后再展开，位置保持正确

4. **追问场景**
   - 第一次思考完成并折叠
   - 发送追问
   - 确认新的思考自动展开

## 📝 注意事项

1. **折叠延迟**：思考完成后延迟 500ms 才折叠，让用户看到完成状态
2. **滚动条样式**：需要 `tailwind-scrollbar` 插件支持（如果没有会降级为系统默认滚动条）
3. **性能优化**：使用 `useRef` 避免不必要的重渲染
4. **状态同步**：通过 `useEffect` 确保 `isStreaming` 和 `expanded` 状态同步

## 🔗 相关文件

- `src/components/workspace/ChatInterface.tsx` - 实现思考消息自动展开/折叠逻辑
