import React, { ReactElement } from 'react';
import { Button } from '@/components/ui/button';

interface ActionButtonProps {
  children: React.ReactNode;
  label: boolean;
  icon: ReactElement;
  disabled: boolean;
  onClick: () => void;
}

export default function ActionButton({
  label,
  children,
  icon,
  disabled,
  onClick,
}: ActionButtonProps) {
  return label ? (
    <Button
      variant="outline"
      size="sm"
      disabled={disabled}
      onClick={onClick}
      className="flex items-center gap-2 cursor-pointer"
    >
      <span className="h-4 w-4">{icon}</span>
      {children}
    </Button>
  ) : (
    <Button
      variant="ghost"
      size="icon"
      disabled={disabled}
      onClick={onClick}
      className="h-8 w-8 cursor-pointer"
    >
      <span className="h-4 w-4">{icon}</span>
      <span className="sr-only">{children}</span>
    </Button>
  );
}
