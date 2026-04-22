import { Button } from '@/components/ui/button';
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { useCanWrite } from '@/contexts/AuthContext';
import { useConfig } from '@/contexts/ConfigContext';
import { Wand2 } from 'lucide-react';
import React from 'react';
import { Link } from 'react-router-dom';
import { RefreshButton } from '../../../../components/ui/refresh-button';
import Title from '../../../../ui/Title';
import { CreateDAGButton } from '../common';

interface DAGListHeaderProps {
  onRefresh?: () => void | Promise<void>;
}

function OpenWorkflowDesignButton() {
  const canWrite = useCanWrite();
  const config = useConfig();

  if (!canWrite || !config.agentEnabled) {
    return null;
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          asChild
          variant="outline"
          size="icon"
          aria-label="Open workflow design"
          title="Open workflow design"
        >
          <Link to="/design">
            <Wand2 className="h-4 w-4" />
          </Link>
        </Button>
      </TooltipTrigger>
      <TooltipContent>Open workflow design</TooltipContent>
    </Tooltip>
  );
}

const DAGListHeader: React.FC<DAGListHeaderProps> = ({ onRefresh }) => (
  <div className="flex flex-row items-center justify-between mb-2">
    <Title>DAG Definitions</Title>
    <div className="flex gap-2">
      <CreateDAGButton />
      <OpenWorkflowDesignButton />
      {onRefresh && <RefreshButton onRefresh={onRefresh} />}
    </div>
  </div>
);

export default DAGListHeader;
