import React from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";

import { cn } from "@/lib/utils";

interface PanelDividerHandleProps {
  collapsed?: boolean;
  onCollapsedChange?: (collapsed: boolean) => void;
  collapsible?: boolean;
  collapseTitleCollapsed?: string;
  collapseTitleExpanded?: string;
  draggable?: boolean;
  size?: number;
  onSizeChange?: (size: number) => void;
  minSize?: number;
  maxSize?: number;
  resizeSign?: 1 | -1;
  hidden?: boolean;
  hideToggleButton?: boolean;
  className?: string;
}

export function PanelDividerHandle({
  collapsed = false,
  onCollapsedChange,
  collapsible = true,
  collapseTitleCollapsed = "展开侧栏",
  collapseTitleExpanded = "折叠侧栏",
  draggable = false,
  size = 0,
  onSizeChange,
  minSize = 220,
  maxSize = 640,
  resizeSign = 1,
  hidden = false,
  hideToggleButton = false,
  className,
}: PanelDividerHandleProps) {
  const resizeStateRef = React.useRef<{ startX: number; startSize: number } | null>(null);
  const [isDragging, setIsDragging] = React.useState(false);

  React.useEffect(() => {
    const handlePointerMove = (event: PointerEvent) => {
      const state = resizeStateRef.current;
      if (!state || !draggable || !onSizeChange) return;
      const delta = (event.clientX - state.startX) * resizeSign;
      const nextSize = Math.max(minSize, Math.min(maxSize, state.startSize + delta));
      onSizeChange(nextSize);
    };

    const handlePointerUp = () => {
      resizeStateRef.current = null;
      setIsDragging(false);
      document.body.style.removeProperty("user-select");
      document.body.style.removeProperty("cursor");
    };

    window.addEventListener("pointermove", handlePointerMove);
    window.addEventListener("pointerup", handlePointerUp);
    return () => {
      window.removeEventListener("pointermove", handlePointerMove);
      window.removeEventListener("pointerup", handlePointerUp);
      document.body.style.removeProperty("user-select");
      document.body.style.removeProperty("cursor");
    };
  }, [draggable, maxSize, minSize, onSizeChange, resizeSign]);

  const handlePointerDown = React.useCallback((event: React.PointerEvent<HTMLDivElement>) => {
    if (!draggable || !onSizeChange || hidden) return;
    event.preventDefault();
    resizeStateRef.current = {
      startX: event.clientX,
      startSize: size,
    };
    setIsDragging(true);
    document.body.style.userSelect = "none";
    document.body.style.cursor = "col-resize";
  }, [draggable, hidden, onSizeChange, size]);

  const handleToggle = React.useCallback(() => {
    if (!collapsible || !onCollapsedChange) return;
    onCollapsedChange(!collapsed);
  }, [collapsed, collapsible, onCollapsedChange]);

  return (
    <div
      className={cn(
        "relative shrink-0 border-r border-border/70 bg-background/40 transition-[width,opacity,background-color] duration-200 ease-out hover:bg-accent/30",
        hidden ? "w-0 opacity-0 border-r-0" : "w-3 opacity-100",
        draggable && "cursor-col-resize",
        isDragging && "bg-accent/40",
        className,
      )}
      onPointerDown={handlePointerDown}
    >
      {collapsible && !hideToggleButton ? (
        <button
          type="button"
          className={cn(
            "absolute left-1/2 top-1/2 z-10 flex h-16 w-4 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full border border-border/70 bg-background/95 text-muted-foreground shadow-sm transition-colors hover:bg-accent hover:text-foreground",
            hidden && "hidden",
          )}
          onClick={handleToggle}
          aria-label={collapsed ? collapseTitleCollapsed : collapseTitleExpanded}
          title={collapsed ? collapseTitleCollapsed : collapseTitleExpanded}
        >
          {collapsed ? <ChevronRight className="h-3.5 w-3.5" /> : <ChevronLeft className="h-3.5 w-3.5" />}
        </button>
      ) : null}
    </div>
  );
}
