import React from 'react';
import { Link } from 'react-router-dom';
import { Tab } from '@/components/ui/tabs';
import { cn } from '@/lib/utils';

export interface LinkTabProps {
  label?: string;
  value: string;
  isActive?: boolean;
  className?: string;
}

const LinkTab: React.FC<LinkTabProps> = ({
  value,
  label,
  isActive,
  className,
  ...props
}) => (
  <Link to={value} className="focus:outline-none">
    <Tab
      value={value}
      isActive={isActive}
      className={cn(
        'rounded-none border-b-2 border-transparent px-4 bg-transparent text-primary',
        isActive && 'border-primary',
        className
      )}
      {...props}
    >
      {label}
    </Tab>
  </Link>
);

export default LinkTab;
