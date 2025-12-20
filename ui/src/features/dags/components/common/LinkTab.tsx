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
        'group relative rounded-md px-3 py-1.5 transition-all duration-200 ease-in-out',
        'flex items-center gap-2 text-sm font-medium cursor-pointer',
        isActive
          ? 'text-amber-700 font-semibold [&_svg]:text-amber-700'
          : 'text-gray-700 hover:text-gray-900',
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
