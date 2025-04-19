import * as React from 'react';
import { cn } from '@/lib/utils';

interface SubTitleProps {
  children?: React.ReactNode;
  className?: string;
}

export default function SubTitle({ children, className }: SubTitleProps) {
  return (
    <h3 className={cn('text-lg font-bold text-[#404040] mb-2', className)}>
      {children}
    </h3>
  );
}
