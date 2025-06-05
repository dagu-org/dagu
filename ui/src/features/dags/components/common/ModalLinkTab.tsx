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
      isActive={isActive}
      className={cn(
        'group relative rounded-md px-4 py-2 transition-all duration-200 ease-in-out',
        'flex items-center gap-2 text-sm font-medium cursor-pointer',
        isActive
          ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border border-blue-200 dark:border-blue-600 shadow-sm font-semibold'
          : 'hover:bg-blue-50 dark:hover:bg-blue-900/30 hover:text-blue-700 dark:hover:text-blue-300 border border-transparent',
        className
      )}
      {...props}
    >
      {Icon && (
        <Icon
          className={cn(
            'h-4 w-4 transition-transform',
            isActive
              ? 'text-blue-700 dark:text-blue-300 scale-110'
              : 'text-blue-600 dark:text-blue-400 group-hover:text-blue-700 dark:group-hover:text-blue-300'
          )}
        />
      )}
      {label}
    </Tab>
  </div>
);

export default ModalLinkTab;
