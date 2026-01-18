import { Tab } from '@/components/ui/tabs';
import { cn } from '@/lib/utils';
import { LucideIcon } from 'lucide-react';
import React from 'react';

export interface ModalLinkTabProps {
  label?: string;
  value?: string;
  isActive?: boolean;
  className?: string;
  icon?: LucideIcon;
  onClick?: () => void;
  'aria-label'?: string;
}

const ModalLinkTab: React.FC<ModalLinkTabProps> = ({
  label,
  isActive,
  className,
  icon: Icon,
  onClick,
  'aria-label': ariaLabel,
  ...props
}) => (
  <button
    type="button"
    className="focus:outline-none cursor-pointer bg-transparent border-none p-0"
    onClick={onClick}
    aria-label={!label ? ariaLabel : undefined}
  >
    <Tab
      isActive={isActive}
      asChild
      className={cn(
        'flex items-center gap-2 text-sm font-medium cursor-pointer',
        className
      )}
      {...props}
    >
      {Icon && <Icon className="h-4 w-4" aria-hidden="true" />}
      {label}
    </Tab>
  </button>
);

export default ModalLinkTab;
