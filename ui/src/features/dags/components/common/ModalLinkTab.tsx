import { Tab } from '@/components/ui/tabs';
import { cn } from '@/lib/utils';
import { LucideIcon } from 'lucide-react';
import React from 'react';

export interface ModalLinkTabProps {
  label?: string;
  value: string;
  isActive?: boolean;
  className?: string;
  icon?: LucideIcon;
  onClick?: () => void;
}

const ModalLinkTab: React.FC<ModalLinkTabProps> = ({
  value,
  label,
  isActive,
  className,
  icon: Icon,
  onClick,
  ...props
}) => (
  <div className="focus:outline-none cursor-pointer" onClick={onClick}>
    <Tab
      value={value}
      isActive={isActive}
      className={cn(
        'group relative rounded-md px-4 py-2 transition-all duration-200 ease-in-out',
        'flex items-center gap-2 text-sm font-medium cursor-pointer',
        isActive
          ? 'bg-primary/10 text-primary border border-primary/30 shadow-sm font-semibold'
          : 'hover:bg-primary/5 hover:text-primary border border-transparent',
        className
      )}
      {...props}
    >
      {Icon && (
        <Icon
          className={cn(
            'h-4 w-4 transition-transform',
            isActive
              ? 'text-primary scale-110'
              : 'text-primary/60 group-hover:text-primary/80'
          )}
        />
      )}
      {label}
    </Tab>
  </div>
);

export default ModalLinkTab;
