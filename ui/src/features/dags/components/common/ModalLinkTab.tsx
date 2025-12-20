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
}

const ModalLinkTab: React.FC<ModalLinkTabProps> = ({
  label,
  isActive,
  className,
  icon: Icon,
  onClick,
  ...props
}) => (
  <div className="focus:outline-none cursor-pointer" onClick={onClick}>
    <Tab
      isActive={isActive}
      className={cn(
        'group relative rounded-md px-3 py-1.5 transition-all duration-200 ease-in-out',
        'flex items-center gap-2 text-sm font-medium cursor-pointer',
        isActive
          ? 'bg-accent-surface text-foreground font-medium'
          : 'hover:bg-accent-surface text-muted-foreground hover:text-foreground bg-transparent',
        className
      )}
      {...props}
    >
      {Icon && (
        <Icon
          className={cn(
            'h-4 w-4 transition-transform',
            isActive ? 'text-foreground' : 'text-muted-foreground group-hover:text-foreground'
          )}
        />
      )}
      {label}
    </Tab>
  </div>
);

export default ModalLinkTab;
