import type { ToolViewerProps } from './index';

export function DefaultToolViewer({ args }: ToolViewerProps): React.ReactNode {
  return (
    <pre className="text-xs overflow-x-auto whitespace-pre-wrap break-words">
      {JSON.stringify(args, null, 2)}
    </pre>
  );
}
