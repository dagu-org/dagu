import React, { useContext } from 'react';
import { useQuery } from '@/hooks/api';
import { AppBarContext } from '@/contexts/AppBarContext';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';

interface Props {
  selectedTemplate: string;
  onSelect: (fileName: string) => void;
}

export function TemplateSelector({ selectedTemplate, onSelect }: Props): React.ReactElement {
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  const { data } = useQuery('/dags', {
    params: {
      query: { remoteNode, perPage: 100 },
    },
  });

  const dags = data?.dags ?? [];

  return (
    <Select value={selectedTemplate || '__none__'} onValueChange={(v) => onSelect(v === '__none__' ? '' : v)}>
      <SelectTrigger className="h-7 text-xs w-48">
        <SelectValue placeholder="Select template" />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="__none__">
          <span className="text-muted-foreground">Select template...</span>
        </SelectItem>
        {dags.map((dag) => (
          <SelectItem key={dag.fileName} value={dag.fileName}>
            {dag.dag.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
