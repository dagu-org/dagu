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
}

const LinkTab: React.FC<LinkTabProps> = ({
  value,
  label,
  isActive,
  className,
  icon: Icon,
  ...props
}) => (
  <Link to={value} className="focus:outline-none cursor-pointer">
    <Tab
      isActive={isActive}
      className={cn(
        'flex items-center gap-2 text-sm font-medium cursor-pointer',
        className
      )}
      {...props}
    >
      {Icon && <Icon className="h-4 w-4" />}
      {label}
    </Tab>
  </Link>
);

export default LinkTab;
