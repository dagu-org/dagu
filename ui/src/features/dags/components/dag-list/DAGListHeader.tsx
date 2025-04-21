import React from 'react';
import Title from '../../../../ui/Title';
import { CreateDAGButton } from '../common';

const DAGListHeader: React.FC = () => (
  <div className="flex flex-row items-center justify-between mb-2">
    <Title className="text-xl mb-0">DAGs</Title>
    <CreateDAGButton />
  </div>
);

export default DAGListHeader;
