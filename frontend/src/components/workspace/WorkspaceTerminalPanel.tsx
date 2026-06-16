import { useEffect, useMemo, useRef } from "react";
import { FitAddon } from "@xterm/addon-fit";
import { Terminal as XTerm } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";

import { api } from "@/lib/api";

interface WorkspaceTerminalPanelProps {
  sessionId: string;
  visible: boolean;
}

export function WorkspaceTerminalPanel({ sessionId, visible }: WorkspaceTerminalPanelProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const terminalRef = useRef<XTerm | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const cursorRef = useRef(0);
  const inputBufferRef = useRef("");
  const resizeFrameRef = useRef<number | null>(null);

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
      cursorBlink: true,
      cursorStyle: "block",
      disableStdin: false,
      fontFamily:
        '"0xProto Mono", ui-monospace, "SF Mono", SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace',
      fontSize: 13,
      lineHeight: 1.25,
      scrollback: 5000,
      theme,
    });
    const fitAddon = new FitAddon();

    terminal.loadAddon(fitAddon);
    terminal.open(containerRef.current);
    fitAddon.fit();

    terminalRef.current = terminal;
    fitAddonRef.current = fitAddon;

    const onDataDispose = terminal.onData((data) => {
      inputBufferRef.current += data;
    });

    const resizeObserver = new ResizeObserver(() => {
      if (resizeFrameRef.current != null) {
        cancelAnimationFrame(resizeFrameRef.current);
      }
      resizeFrameRef.current = requestAnimationFrame(() => {
        fitAddonRef.current?.fit();
        const activeTerminal = terminalRef.current;
        if (!activeTerminal) return;
        void api.resizeTerminalSession(sessionId, {
          cols: activeTerminal.cols,
          rows: activeTerminal.rows,
        });
      });
    });
    resizeObserver.observe(containerRef.current);

    return () => {
      onDataDispose.dispose();
      resizeObserver.disconnect();
      if (resizeFrameRef.current != null) {
        cancelAnimationFrame(resizeFrameRef.current);
      }
      terminal.dispose();
      terminalRef.current = null;
      fitAddonRef.current = null;
    };
  }, [sessionId, theme]);

  useEffect(() => {
    cursorRef.current = 0;
    inputBufferRef.current = "";
    const terminal = terminalRef.current;
    if (terminal) {
      terminal.reset();
    }
  }, [sessionId]);

  useEffect(() => {
    if (!visible) return;
    fitAddonRef.current?.fit();
    const terminal = terminalRef.current;
    if (!terminal) return;
    void api.resizeTerminalSession(sessionId, {
      cols: terminal.cols,
      rows: terminal.rows,
    });
  }, [sessionId, visible]);

  useEffect(() => {
    const poll = window.setInterval(async () => {
      try {
        const data = await api.pollTerminalSession(sessionId, cursorRef.current);
        if (data.output) {
          terminalRef.current?.write(data.output);
        }
        cursorRef.current = data.cursor;
      } catch {
        // keep polling; terminal panel stays mounted while hidden
      }
    }, 250);
    return () => window.clearInterval(poll);
  }, [sessionId]);

  useEffect(() => {
    const flush = window.setInterval(() => {
      const value = inputBufferRef.current;
      if (!value) return;
      inputBufferRef.current = "";
      void api.writeTerminalSession(sessionId, value);
    }, 40);
    return () => window.clearInterval(flush);
  }, [sessionId]);

  return (
    <div className="flex min-h-0 flex-1 flex-col bg-slate-950">
      <div ref={containerRef} className="min-h-0 flex-1 overflow-hidden px-2 py-2" />
    </div>
  );
}