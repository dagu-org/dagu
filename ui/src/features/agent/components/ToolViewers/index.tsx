import { BashToolViewer } from './BashToolViewer';
import { ReadToolViewer } from './ReadToolViewer';
import { PatchToolViewer } from './PatchToolViewer';
import { ThinkToolViewer } from './ThinkToolViewer';
import { NavigateToolViewer } from './NavigateToolViewer';
import { ReadSchemaToolViewer } from './ReadSchemaToolViewer';
import { AskUserToolViewer } from './AskUserToolViewer';
import { DefaultToolViewer } from './DefaultToolViewer';

export interface ToolViewerProps {
  toolName: string;
  args: Record<string, unknown>;
}

const toolViewerRegistry: Record<string, React.FC<ToolViewerProps>> = {
  bash: BashToolViewer,
  read: ReadToolViewer,
  patch: PatchToolViewer,
  think: ThinkToolViewer,
  navigate: NavigateToolViewer,
  read_schema: ReadSchemaToolViewer,
  ask_user: AskUserToolViewer,
};

export function ToolContentViewer({ toolName, args }: ToolViewerProps): React.ReactNode {
  const Viewer = toolViewerRegistry[toolName] || DefaultToolViewer;
  return <Viewer args={args} toolName={toolName} />;
}

export { BashToolViewer } from './BashToolViewer';
export { ReadToolViewer } from './ReadToolViewer';
export { PatchToolViewer } from './PatchToolViewer';
export { ThinkToolViewer } from './ThinkToolViewer';
export { NavigateToolViewer } from './NavigateToolViewer';
export { ReadSchemaToolViewer } from './ReadSchemaToolViewer';
export { AskUserToolViewer } from './AskUserToolViewer';
export { DefaultToolViewer } from './DefaultToolViewer';
