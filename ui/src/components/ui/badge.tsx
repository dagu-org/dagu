import * as React from 'react';
import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';

import { cn } from '@/lib/utils';

const badgeVariants = cva(
  'inline-flex h-5 w-fit shrink-0 items-center justify-center gap-1 overflow-hidden whitespace-nowrap rounded-md border px-1.5 text-[11px] font-medium transition-colors [&>svg]:pointer-events-none [&>svg]:size-3',
  {
    variants: {
      variant: {
        default: 'bg-muted text-muted-foreground border-transparent',
        primary: 'bg-primary/10 text-primary border-transparent',
        secondary: 'bg-secondary text-secondary-foreground border-transparent',
        success:
          'bg-success/10 text-success border-transparent dark:bg-success/15',
        error:
          'bg-destructive/10 text-destructive border-transparent dark:bg-destructive/15',
        warning:
          'bg-warning/10 text-warning border-transparent dark:bg-warning/15',
        info: 'bg-info/10 text-info border-transparent dark:bg-info/15',
        outline:
          'bg-transparent text-foreground border-border [a&]:hover:bg-muted',
        // Status-specific variants for DAG runs
        running: 'bg-info/10 text-info border-transparent dark:bg-info/15',
        failed:
          'bg-destructive/10 text-destructive border-transparent dark:bg-destructive/15',
        cancelled: 'bg-muted text-muted-foreground border-transparent',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  }
);

function Badge({
  className,
  variant,
  asChild = false,
  ...props
}: React.ComponentProps<'span'> &
  VariantProps<typeof badgeVariants> & { asChild?: boolean }) {
  const Comp = asChild ? Slot : 'span';

  return (
    <Comp
      data-slot="badge"
      className={cn(badgeVariants({ variant }), className)}
      {...props}
    />
  );
}

export { Badge, badgeVariants };
