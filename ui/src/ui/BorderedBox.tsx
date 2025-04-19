import React from 'react';
import { Card } from '@/components/ui/card';
import { cn } from '@/lib/utils';

interface BorderedBoxProps extends React.ComponentProps<typeof Card> {
  children?: React.ReactNode;
  sx?: Record<string, any>; // Keep sx prop for backward compatibility
}

export default function BorderedBox({
  children,
  className,
  sx,
  ...props
}: BorderedBoxProps) {
  // Convert common MUI sx properties to Tailwind classes
  let sxClasses = '';

  if (sx) {
    // Handle margin top
    if (sx.mt !== undefined) {
      sxClasses += ` mt-${sx.mt * 2}`;
    }

    // Handle padding
    if (sx.py !== undefined) {
      sxClasses += ` py-${sx.py * 2}`;
    }

    if (sx.px !== undefined) {
      sxClasses += ` px-${sx.px * 2}`;
    }

    // Handle display
    if (sx.display === 'flex') {
      sxClasses += ' flex';
    }

    // Handle flex direction
    if (sx.flexDirection === 'column') {
      sxClasses += ' flex-col';
    }

    // Handle overflow
    if (sx.overflowX === 'auto') {
      sxClasses += ' overflow-x-auto';
    }
  }
  return (
    <Card
      className={cn(
        'rounded-sm border-[#e0e0e0] bg-white shadow-none py-0',
        sxClasses,
        className
      )}
      {...props}
    >
      {children}
    </Card>
  );
}
