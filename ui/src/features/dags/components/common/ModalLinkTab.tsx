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
          ? 'text-primary font-semibold [&_svg]:text-primary'
          : 'text-foreground hover:text-foreground/80',
        className
      )}
      {...props}
    >
      {Icon && <Icon className="h-4 w-4" />}
      {label}
    </Tab>
  </div>
);

export default ModalLinkTab;
