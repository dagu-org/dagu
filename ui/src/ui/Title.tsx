import { cn } from '@/lib/utils';
import * as React from 'react';

interface TitleProps {
  children?: React.ReactNode;
  className?: string;
}

export default function Title({ children, className }: TitleProps) {
  return (
    <h2
      className={cn(
        'text-2xl font-bold text-foreground mb-4',
        className
      )}
    >
      {children}
    </h2>
  );
}
