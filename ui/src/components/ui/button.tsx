import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';
import * as React from 'react';

import { cn } from '@/lib/utils';

// GCP-Style Button Variants - Flat & Professional
const buttonVariants = cva(
  'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-all duration-150 disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0 shrink-0 outline-none cursor-pointer border',
  {
    variants: {
      variant: {
        default: 'bg-transparent border-border text-foreground hover:bg-muted hover:border-border-strong',
        primary: 'bg-primary border-primary text-primary-foreground hover:bg-primary-hover hover:border-primary-hover dark:bg-transparent dark:text-primary dark:hover:bg-primary/10',
        destructive: 'bg-destructive border-destructive text-destructive-foreground hover:opacity-90',
        outline: 'bg-transparent border-primary text-primary hover:bg-primary/8',
        secondary: 'bg-secondary border-secondary text-secondary-foreground hover:bg-secondary/80',
        ghost: 'bg-transparent border-transparent text-text-secondary hover:bg-muted hover:text-foreground',
        link: 'bg-transparent border-transparent text-primary hover:underline hover:text-primary-hover',
      },
      size: {
        default: 'h-9 px-4',
        sm: 'h-8 px-3 text-xs',
        xs: 'h-7 px-2 text-xs',
        lg: 'h-10 px-5',
        icon: 'size-9 p-0',
        'icon-sm': 'size-7 p-0',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  }
);

function Button({
  className,
  variant,
  size,
  asChild = false,
  ...props
}: React.ComponentProps<'button'> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean;
  }) {
  const Comp = asChild ? Slot : 'button';

  return (
    <Comp
      data-slot="button"
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  );
}

export { Button, buttonVariants };
