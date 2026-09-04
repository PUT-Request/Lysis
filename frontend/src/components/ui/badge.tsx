import * as React from "react"
import { cn } from "@/lib/utils"

function Badge({
  className,
  variant = "default",
  ...props
}: React.HTMLAttributes<HTMLDivElement> & {
  variant?: "default" | "secondary" | "destructive" | "outline" | "success" | "warning"
}) {
  const variants: Record<string, string> = {
    default: "bg-primary text-primary-foreground",
    secondary: "bg-muted text-muted-foreground",
    destructive: "bg-destructive text-destructive-foreground",
    outline: "border text-foreground",
    success: "bg-success text-white",
    warning: "bg-warning text-black",
  }
  return (
    <div
      className={cn(
        "inline-flex items-center rounded-[4px] px-2 py-0.5 text-xs font-medium",
        variants[variant],
        className
      )}
      {...props}
    />
  )
}

export { Badge }
