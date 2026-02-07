import React from 'react';
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '../../../../components/ui/breadcrumb';
import { RefreshButton } from '../../../../components/ui/refresh-button';
import Title from '../../../../ui/Title';
import { CreateDAGButton } from '../common';

interface DAGListHeaderProps {
  onRefresh?: () => void | Promise<void>;
  namespace?: string;
}

function DAGListHeader({ onRefresh, namespace }: DAGListHeaderProps): React.ReactElement {
  return (
    <div className="flex flex-col gap-1 mb-2">
      {namespace && (
        <Breadcrumb>
          <BreadcrumbList>
            <BreadcrumbItem>
              <BreadcrumbPage>{namespace}</BreadcrumbPage>
            </BreadcrumbItem>
            <BreadcrumbSeparator />
            <BreadcrumbItem>
              <BreadcrumbPage>DAGs</BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>
      )}
      <div className="flex flex-row items-center justify-between">
        <Title>DAG Definitions</Title>
        <div className="flex gap-2">
          <CreateDAGButton />
          {onRefresh && <RefreshButton onRefresh={onRefresh} />}
        </div>
      </div>
    </div>
  );
}

export default DAGListHeader;
