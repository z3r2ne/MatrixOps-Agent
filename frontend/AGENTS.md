# Repository Guidelines

## Project Structure & Module Organization
- `backend/`: Go + Gin API server, database setup, handlers, and models. Entry at `backend/main.go`.
- `backend/web/dist/`: embedded frontend build output (generated; do not edit).
- `components/`: shared React UI pieces; `components/ui/` is shadcn/ui plus layout/forms.
- `views/`: screen-level React pages such as `WorkspacesView.tsx` and `KanbanView.tsx`.
- `lib/`: frontend helpers like `lib/api.ts` and `lib/utils.ts`.
- `App.tsx`, `index.tsx`, `index.css`, `types.ts`: app shell, bootstrap, styles, and shared types.

## Build, Test, and Development Commands
- `yarn install`: install frontend dependencies.
- `yarn dev`: run Vite dev server (frontend on `http://localhost:3000`).
- `yarn build`: build production frontend into `backend/web/dist`.
- `task build`: build frontend and backend (embedded frontend).
- `task start`: build frontend and run the Go API server at `http://localhost:8080`.
- `./start-backend.sh` or `cd backend && go run main.go`: start the Go API server (requires frontend build).
- `cd backend && go build -o matrixops`: compile backend binary.

## Coding Style & Naming Conventions
- TypeScript + React with functional components; keep JSX readable and prefer hooks.
- Observed style: 2-space indentation, single quotes, semicolons.
- Components use `PascalCase.tsx`; hooks and utilities use `camelCase`.
- Tailwind CSS for layout/styling; add shared UI elements under `components/ui`.
- Go code relies on `gofmt` formatting and `*_test.go` naming.

## Testing Guidelines
- No automated test scripts are configured yet (`package.json` has no `test`).
- If adding frontend tests, use `*.test.tsx` and document the runner you add.
- For backend tests, use standard Go tests and run `go test ./...`.

## Commit & Pull Request Guidelines
- No Git history is available in this checkout, so no established commit convention was found.
- Prefer concise, present-tense summaries like `feat: add workspace filters`.
- PRs should include: purpose, screenshots for UI changes, and any config or data impacts.

## Configuration & Data
- Frontend uses `VITE_API_URL` in `.env` for API routing (defaults to `/api` when unset).
- SQLite data lives at `~/.matrixops/matrixops.db` by default.
