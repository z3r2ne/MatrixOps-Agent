import * as React from "react"

import { cn } from "@/lib/utils"

const InputGroup = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
  <div
    ref={ref}
    data-slot="input-group"
    className={cn(
      "group/input-group flex h-9 w-full items-center rounded-md border border-input bg-background text-sm shadow-sm transition-colors focus-within:ring-1 focus-within:ring-ring has-[:disabled]:cursor-not-allowed has-[:disabled]:opacity-50",
      className
    )}
    {...props}
  />
))
InputGroup.displayName = "InputGroup"

const InputGroupInput = React.forwardRef<
  HTMLInputElement,
  React.InputHTMLAttributes<HTMLInputElement>
>(({ className, ...props }, ref) => (
  <input
    ref={ref}
    data-slot="input-group-input"
    className={cn(
      "min-w-0 flex-1 bg-transparent px-3 py-1 outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed",
      className
    )}
    {...props}
  />
))
InputGroupInput.displayName = "InputGroupInput"

function mergeRefs<T>(
  ...refs: Array<React.Ref<T> | undefined>
): React.RefCallback<T> {
  return (value) => {
    refs.forEach((ref) => {
      if (!ref) return
      if (typeof ref === "function") {
        ref(value)
        return
      }
      try {
        ;(ref as React.MutableRefObject<T | null>).current = value
      } catch {
        // noop
      }
    })
  }
}

type InputGroupButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "ghost" | "default"
  size?: "icon-xs" | "sm" | "default"
  render?: React.ReactElement
}

const InputGroupButton = React.forwardRef<HTMLButtonElement, InputGroupButtonProps>(
  ({ className, variant = "default", size = "default", render, children, ...props }, ref) => {
    const classes = cn(
      "inline-flex shrink-0 items-center justify-center rounded-sm font-medium transition-colors focus-visible:outline-none disabled:pointer-events-none disabled:opacity-50",
      variant === "ghost" ? "hover:bg-accent hover:text-accent-foreground" : "bg-transparent",
      size === "icon-xs" ? "size-6" : size === "sm" ? "h-7 px-2 text-xs" : "h-8 px-2",
      className
    )

    if (render) {
      return React.cloneElement(render, {
        ...props,
        ref: mergeRefs((render as any).ref, ref),
        className: cn((render.props as { className?: string }).className, classes),
        children: children ?? render.props.children,
      } as any)
    }

    return (
      <button ref={ref} className={classes} type="button" {...props}>
        {children}
      </button>
    )
  }
)
InputGroupButton.displayName = "InputGroupButton"

const InputGroupAddon = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement> & { align?: "inline-start" | "inline-end" }
>(({ className, align = "inline-start", ...props }, ref) => (
  <div
    ref={ref}
    data-slot="input-group-addon"
    data-align={align}
    className={cn(
      "flex items-center px-1",
      align === "inline-start" ? "order-first" : "order-last",
      className
    )}
    {...props}
  />
))
InputGroupAddon.displayName = "InputGroupAddon"

export { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput }

