export const DEFAULT_PROJECT_TOOL_PERMISSIONS_CONFIG_KEY = "default_project_tool_permissions"

export function parseProjectToolPermissions(raw?: string | null): Record<string, string> {
  if (!raw?.trim()) {
    return {}
  }

  try {
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return {}
    }
    return Object.entries(parsed).reduce<Record<string, string>>((acc, [key, value]) => {
      if (typeof value === "string") {
        acc[key] = value
      }
      return acc
    }, {})
  } catch {
    return {}
  }
}

export function serializeProjectToolPermissions(values: Record<string, string>): string {
  return JSON.stringify(values)
}

export function cloneProjectToolPermissions(values: Record<string, string>): Record<string, string> {
  return { ...values }
}
