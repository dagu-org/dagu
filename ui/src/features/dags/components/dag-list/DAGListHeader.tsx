import React from 'react';
import { RefreshButton } from '../../../../components/ui/refresh-button';
import Title from '../../../../ui/Title';
import { CreateDAGButton } from '../common';

interface DAGListHeaderProps {
  onRefresh?: () => void | Promise<void>;
}

const DAGListHeader: React.FC<DAGListHeaderProps> = ({ onRefresh }) => (
  <div className="flex flex-row items-center justify-between mb-2 mr-2">
    <Title className="text-xl mb-0">DAG Definitions</Title>
    <div className="flex gap-2">
      <CreateDAGButton />
      {onRefresh && <RefreshButton onRefresh={onRefresh} />}
    </div>
  </div>
);

export default DAGListHeader;
