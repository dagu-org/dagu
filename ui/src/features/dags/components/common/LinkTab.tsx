import { Tab } from '@/components/ui/tabs';
import { cn } from '@/lib/utils';
import { LucideIcon } from 'lucide-react';
import React from 'react';
import { Link } from 'react-router-dom';

export interface LinkTabProps {
  label?: string;
  value: string;
  isActive?: boolean;
  className?: string;
  icon?: LucideIcon;
  'aria-label'?: string;
}

const LinkTab: React.FC<LinkTabProps> = ({
  value,
  label,
  isActive,
  className,
  icon: Icon,
  'aria-label': ariaLabel,
  ...props
}) => (
  <Link
    to={value}
    className="focus:outline-none cursor-pointer"
    aria-label={!label ? ariaLabel : undefined}
  >
    <Tab
      isActive={isActive}
      className={cn(
        'flex items-center gap-2 text-sm font-medium cursor-pointer',
        className
      )}
      {...props}
    >
      {Icon && <Icon className="h-4 w-4" aria-hidden="true" />}
      {label}
    </Tab>
  </Link>
);

export default LinkTab;
