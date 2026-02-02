import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

// GCP-Style Badge Variants - Professional Status Indicators
const badgeVariants = cva(
  "inline-flex items-center justify-center rounded-sm border px-2 h-5 text-[11px] font-medium w-fit whitespace-nowrap shrink-0 [&>svg]:size-3 gap-1 [&>svg]:pointer-events-none transition-colors uppercase tracking-wide overflow-hidden",
  {
    variants: {
      variant: {
        default:
          "bg-muted text-muted-foreground border-border",
        primary:
          "bg-primary/10 text-primary border-primary/20",
        secondary:
          "bg-secondary text-secondary-foreground border-border",
        success:
          "bg-success/10 text-success border-success/20 dark:bg-success/15 dark:text-success dark:border-success/30",
        error:
          "bg-destructive/10 text-destructive border-destructive/20 dark:bg-destructive/15 dark:text-destructive dark:border-destructive/30",
        warning:
          "bg-warning/10 text-warning border-warning/20 dark:bg-warning/15 dark:text-warning dark:border-warning/30",
        info:
          "bg-info/10 text-info border-info/20 dark:bg-info/15 dark:text-info dark:border-info/30",
        outline:
          "bg-transparent text-foreground border-border [a&]:hover:bg-muted",
        // Status-specific variants for DAG runs
        running:
          "bg-info/10 text-info border-info/20 dark:bg-info/15 dark:text-info dark:border-info/30",
        failed:
          "bg-destructive/10 text-destructive border-destructive/20 dark:bg-destructive/15 dark:text-destructive dark:border-destructive/30",
        cancelled:
          "bg-muted text-muted-foreground border-border",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

function Badge({
  className,
  variant,
  asChild = false,
  ...props
}: React.ComponentProps<"span"> &
  VariantProps<typeof badgeVariants> & { asChild?: boolean }) {
  const Comp = asChild ? Slot : "span"

  return (
    <Comp
      data-slot="badge"
      className={cn(badgeVariants({ variant }), className)}
      {...props}
    />
  )
}

export { Badge, badgeVariants }
