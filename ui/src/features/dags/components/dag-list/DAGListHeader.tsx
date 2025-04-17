import React from 'react';
import Title from '../../../../ui/Title';
import { CreateDAGButton } from '../common';

const DAGListHeader: React.FC = () => (
  // Replace MUI Box with div and Tailwind classes
  <div className="flex flex-row items-center justify-between mb-4">
    {' '}
    {/* Added margin-bottom for spacing */}
    <Title>DAGs</Title>
    <CreateDAGButton />
  </div>
);

export default DAGListHeader;
