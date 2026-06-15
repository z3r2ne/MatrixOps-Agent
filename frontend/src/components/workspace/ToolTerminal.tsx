import { useEffect, useMemo, useRef } from "react";
import { FitAddon } from "@xterm/addon-fit";
import { Terminal as XTerm } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";

interface ToolTerminalProps {
  content: string;
  isRunning?: boolean;
  className?: string;
}

export function ToolTerminal({ content, isRunning = false, className }: ToolTerminalProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const terminalRef = useRef<XTerm | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const lastContentRef = useRef("");

  const theme = useMemo(
    () => ({
      background: "#020617",
      foreground: "#E2E8F0",
      cursor: "#38BDF8",
      selectionBackground: "rgba(56, 189, 248, 0.18)",
      black: "#0F172A",
      brightBlack: "#475569",
      red: "#F87171",
      brightRed: "#FCA5A5",
      green: "#4ADE80",
      brightGreen: "#86EFAC",
      yellow: "#FACC15",
      brightYellow: "#FDE047",
      blue: "#60A5FA",
      brightBlue: "#93C5FD",
      magenta: "#C084FC",
      brightMagenta: "#D8B4FE",
      cyan: "#22D3EE",
      brightCyan: "#67E8F9",
      white: "#E2E8F0",
      brightWhite: "#F8FAFC",
    }),
    []
  );

  useEffect(() => {
    if (!containerRef.current || terminalRef.current) return;

    const terminal = new XTerm({
      allowTransparency: false,
      convertEol: true,
      cursorBlink: isRunning,
      cursorStyle: "block",
      disableStdin: true,
      fontFamily: '"0xProto Mono", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
      fontSize: 13,
      lineHeight: 1.35,
      rows: 12,
      scrollback: 3000,
      theme,
    });
    const fitAddon = new FitAddon();

    terminal.loadAddon(fitAddon);
    terminal.open(containerRef.current);
    fitAddon.fit();

    terminalRef.current = terminal;
    fitAddonRef.current = fitAddon;

    const resizeObserver = new ResizeObserver(() => {
      fitAddonRef.current?.fit();
    });
    resizeObserver.observe(containerRef.current);

    return () => {
      resizeObserver.disconnect();
      terminal.dispose();
      terminalRef.current = null;
      fitAddonRef.current = null;
      lastContentRef.current = "";
    };
  }, [isRunning, theme]);

  useEffect(() => {
    if (!terminalRef.current) return;
    terminalRef.current.options.cursorBlink = isRunning;
    fitAddonRef.current?.fit();
  }, [isRunning]);

  useEffect(() => {
    const terminal = terminalRef.current;
    if (!terminal) return;

    const previous = lastContentRef.current;
    if (!content) {
      terminal.reset();
      lastContentRef.current = "";
      return;
    }

    if (!previous || !content.startsWith(previous)) {
      terminal.reset();
      terminal.write(content);
      lastContentRef.current = content;
      return;
    }

    const delta = content.slice(previous.length);
    if (delta) {
      terminal.write(delta);
      lastContentRef.current = content;
    }
  }, [content]);

  return (
    <div
      ref={containerRef}
      className={className ?? "h-64 w-full overflow-hidden rounded-md border border-slate-800 bg-slate-950 px-1 py-1"}
    />
  );
}