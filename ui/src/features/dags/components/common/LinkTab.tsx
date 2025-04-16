import React from 'react';
import { Link } from 'react-router-dom';
import { Tab } from '@mui/material';

export interface LinkTabProps {
  label?: string;
  value: string;
}

const LinkTab: React.FC<LinkTabProps> = ({ value, ...props }) => (
  <Link to={value}>
    <Tab value={value} {...props} />
  </Link>
);

export default LinkTab;
