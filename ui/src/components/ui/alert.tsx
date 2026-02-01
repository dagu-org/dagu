import * as React from 'react';
import { cva, type VariantProps } from 'class-variance-authority';

import { cn } from '@/lib/utils';

// GCP-Style Alert Variants - Professional Status Colors
const alertVariants = cva(
  'relative w-full rounded-md border p-4 [&>svg~*]:pl-7 [&>svg+div]:translate-y-[-3px] [&>svg]:absolute [&>svg]:left-4 [&>svg]:top-4',
  {
    variants: {
      variant: {
        default: 'bg-muted border-border text-foreground [&>svg]:text-foreground',
        info: 'bg-info/10 border-info/20 text-info [&>svg]:text-info dark:bg-info/15 dark:border-info/30',
        success: 'bg-success/10 border-success/20 text-success [&>svg]:text-success dark:bg-success/15 dark:border-success/30',
        warning: 'bg-warning/10 border-warning/20 text-warning [&>svg]:text-warning dark:bg-warning/15 dark:border-warning/30',
        destructive: 'bg-destructive/10 border-destructive/20 text-destructive [&>svg]:text-destructive dark:bg-destructive/15 dark:border-destructive/30',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  }
);

const Alert = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement> & VariantProps<typeof alertVariants>
>(({ className, variant, ...props }, ref) => (
  <div
    ref={ref}
    role="alert"
    className={cn(alertVariants({ variant }), className)}
    {...props}
  />
));
Alert.displayName = 'Alert';

const AlertTitle = React.forwardRef<
  HTMLParagraphElement,
  React.HTMLAttributes<HTMLHeadingElement>
>(({ className, ...props }, ref) => (
  <h5
    ref={ref}
    className={cn('mb-1 font-medium leading-none tracking-tight', className)}
    {...props}
  />
));
AlertTitle.displayName = 'AlertTitle';

const AlertDescription = React.forwardRef<
  HTMLParagraphElement,
  React.HTMLAttributes<HTMLParagraphElement>
>(({ className, ...props }, ref) => (
  <div
    ref={ref}
    className={cn('text-sm [&_p]:leading-relaxed', className)}
    {...props}
  />
));
AlertDescription.displayName = 'AlertDescription';

export { Alert, AlertTitle, AlertDescription };
