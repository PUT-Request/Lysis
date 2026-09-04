"use client"

import { useEffect, useState, useCallback } from "react"
import { cn } from "@/lib/utils"

export function Dialog({
  open,
  onClose,
  className,
  children,
}: {
  open: boolean
  onClose: () => void
  className?: string
  children: React.ReactNode
}) {
  const [visible, setVisible] = useState(false)
  const [animating, setAnimating] = useState(false)

  useEffect(() => {
    if (open) {
      setVisible(true)
      requestAnimationFrame(() => setAnimating(true))
    } else {
      setAnimating(false)
      const timer = setTimeout(() => setVisible(false), 180)
      return () => clearTimeout(timer)
    }
  }, [open])

  const handleKey = useCallback(
    (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose()
    },
    [onClose]
  )

  useEffect(() => {
    if (open) {
      document.addEventListener("keydown", handleKey)
      document.body.style.overflow = "hidden"
      return () => {
        document.removeEventListener("keydown", handleKey)
        document.body.style.overflow = ""
      }
    }
  }, [open, handleKey])

  if (!visible) return null

  return (
    <div
      role="dialog"
      aria-modal="true"
      className="fixed inset-0 z-50 flex items-center justify-center"
    >
      <div
        className={cn(
          "fixed inset-0 bg-black/50 transition-opacity duration-180",
          animating ? "opacity-100" : "opacity-0"
        )}
        onClick={onClose}
      />
      <div
        className={cn(
          "relative z-50 w-full max-w-lg border bg-card p-6 shadow-lg transition-all duration-200",
          animating ? "opacity-100 scale-100" : "opacity-0 scale-95",
          className
        )}
      >
        {children}
      </div>
    </div>
  )
}

export function DialogHeader({ className, children }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("mb-4", className)}>{children}</div>
}

export function DialogTitle({ className, ...props }: React.HTMLAttributes<HTMLHeadingElement>) {
  return <h2 className={cn("text-sm font-semibold", className)} {...props} />
}
