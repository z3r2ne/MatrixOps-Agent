# AI Message Markdown Plan

## Goal

Render assistant text messages in the workspace chat as Markdown instead of plain pre-wrapped text.

## Scope

- Update the assistant message renderer in `ChatInterfaceV2`.
- Support common Markdown features used by AI output:
  - headings
  - lists
  - links
  - inline code
  - fenced code blocks
  - blockquotes
  - tables
- Keep copy behavior unchanged and avoid rendering raw HTML.

## Validation

- Run `npm run build` in `frontend`.
- Manually verify an assistant reply with lists, code fences, and links in the workspace chat.
