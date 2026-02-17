import { useEffect, useMemo, useRef, useState } from 'react';

import {
  CheckCircle,
  ChevronRight,
  ExternalLink,
  Loader2,
  Terminal,
  XCircle,
} from 'lucide-react';

import { cn } from '@/lib/utils';
import { DelegateInfo, Message, TokenUsage, ToolCall, ToolResult, UIAction, UserPromptResponse } from '../types';
import { formatCost } from '../utils/formatCost';
import { isDelegateToolCall, parseDelegateTasks } from '../utils/delegateUtils';
import { CommandApprovalMessage } from './CommandApprovalMessage';
import { InlineDelegateResult } from './InlineDelegateResult';
import { ToolContentViewer } from './ToolViewers';
import { UserPromptMessage } from './UserPromptMessage';

interface ChatMessagesProps {
  messages: Message[];
  pendingUserMessage: string | null;
  isWorking: boolean;
  onPromptRespond?: (response: UserPromptResponse, displayValue: string) => void;
  answeredPrompts?: Record<string, string>;
  delegateStatuses?: Record<string, DelegateInfo>;
}

export function ChatMessages({
  messages,
  pendingUserMessage,
  isWorking,
  onPromptRespond,
  answeredPrompts,
  delegateStatuses,
}: ChatMessagesProps): React.ReactNode {
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  if (messages.length === 0) {
    return (
      <div className="flex-1 flex items-center justify-center text-muted-foreground p-4 bg-popover">
        <div className="text-center">
          <Terminal className="h-8 w-8 mx-auto mb-2 opacity-30" />
          <p className="text-xs text-muted-foreground">
            Ask the agent to create DAGs, run commands, or help with workflows
          </p>
        </div>
      </div>
    );
  }

  // Check if there's a pending user prompt (not yet answered)
  const hasPendingPrompt = messages.some(
    (m) => m.type === 'user_prompt' && m.user_prompt && !answeredPrompts?.[m.user_prompt.prompt_id]
  );

  return (
    <div className="flex-1 overflow-y-auto p-2 space-y-2 font-mono text-xs bg-popover">
      {messages.map((message) => (
        <MessageItem
          key={message.id}
          message={message}
          onPromptRespond={onPromptRespond}
          answeredPrompts={answeredPrompts}
          delegateStatuses={delegateStatuses}
        />
      ))}
      {pendingUserMessage && (
        <UserMessage content={pendingUserMessage} isPending />
      )}
      {isWorking && !hasPendingPrompt && (
        <div className="flex items-center gap-1.5 text-orange-600 dark:text-orange-400 pl-1">
          <Loader2 className="h-3 w-3 animate-spin" />
          <span>processing...</span>
        </div>
      )}
      <div ref={messagesEndRef} />
    </div>
  );
}

interface MessageItemProps {
  message: Message;
  onPromptRespond?: (response: UserPromptResponse, displayValue: string) => void;
  answeredPrompts?: Record<string, string>;
  delegateStatuses?: Record<string, DelegateInfo>;
}

function MessageItem({ message, onPromptRespond, answeredPrompts, delegateStatuses }: MessageItemProps): React.ReactNode {
  switch (message.type) {
    case 'user':
      // Tool results arrive as "user" type messages with tool_results populated
      if (message.tool_results && message.tool_results.length > 0) {
        if (message.delegate_ids && message.delegate_ids.length > 0) {
          return (
            <DelegateResultsInline
              delegateIds={message.delegate_ids}
              delegateStatuses={delegateStatuses}
            />
          );
        }
        return <ToolResultMessage toolResults={message.tool_results} />;
      }
      return <UserMessage content={message.content ?? ''} />;
    case 'assistant':
      return (
        <AssistantMessage
          content={message.content ?? ''}
          toolCalls={message.tool_calls}
          usage={message.usage}
          cost={message.cost}
        />
      );
    case 'error':
      return <ErrorMessage content={message.content ?? ''} />;
    case 'ui_action':
      return <UIActionMessage action={message.ui_action} />;
    case 'user_prompt':
      if (!message.user_prompt || !onPromptRespond) return null;
      if (message.user_prompt.prompt_type === 'command_approval') {
        return (
          <CommandApprovalMessage
            prompt={message.user_prompt}
            onRespond={onPromptRespond}
            isAnswered={answeredPrompts?.[message.user_prompt.prompt_id] !== undefined}
            answeredValue={answeredPrompts?.[message.user_prompt.prompt_id]}
          />
        );
      }
      return (
        <UserPromptMessage
          prompt={message.user_prompt}
          onRespond={onPromptRespond}
          isAnswered={answeredPrompts?.[message.user_prompt.prompt_id] !== undefined}
          answeredValue={answeredPrompts?.[message.user_prompt.prompt_id]}
        />
      );
    default:
      return null;
  }
}

function ErrorMessage({ content }: { content: string }): React.ReactNode {
  return (
    <div className="pl-1">
      <div className="flex items-start gap-1.5 text-red-500">
        <XCircle className="h-3 w-3 mt-0.5 flex-shrink-0" />
        <p className="whitespace-pre-wrap break-words">{content}</p>
      </div>
    </div>
  );
}

function UIActionMessage({
  action,
}: {
  action?: UIAction;
}): React.ReactNode {
  if (!action || action.type !== 'navigate') {
    return null;
  }

  return (
    <div className="pl-1">
      <div className="flex items-center gap-1.5 text-muted-foreground">
        <ExternalLink className="h-3 w-3 flex-shrink-0" />
        <span>Navigating to {action.path}</span>
      </div>
    </div>
  );
}

function UserMessage({ content, isPending }: { content: string; isPending?: boolean }): React.ReactNode {
  if (!content) return null;

  return (
    <div className="pl-1">
      <div className={cn(
        "inline-flex items-start gap-1.5 px-2.5 py-1.5 rounded-lg",
        "bg-gradient-to-br from-primary/10 to-primary/5 dark:from-primary/20 dark:to-primary/10",
        "text-foreground",
        "border border-primary/20",
        isPending && "opacity-60"
      )}>
        <ChevronRight className="h-3 w-3 mt-0.5 flex-shrink-0 text-primary" />
        <p className="whitespace-pre-wrap break-words">{content}</p>
      </div>
    </div>
  );
}

function AssistantMessage({
  content,
  toolCalls,
  usage,
  cost,
}: {
  content: string;
  toolCalls?: ToolCall[];
  usage?: TokenUsage;
  cost?: number;
}): React.ReactNode {
  // Split tool calls into delegate vs non-delegate
  const hasDelegateCall = isDelegateToolCall(toolCalls);
  const nonDelegateCalls = hasDelegateCall
    ? toolCalls?.filter((tc) => tc.function.name !== 'delegate')
    : toolCalls;

  return (
    <div className="pl-1 space-y-1">
      {content && (
        <p className="whitespace-pre-wrap break-words text-foreground/90 pl-4">
          {content}
        </p>
      )}
      {nonDelegateCalls && nonDelegateCalls.length > 0 && (
        <ToolCallList toolCalls={nonDelegateCalls} className="pl-4" />
      )}
      {hasDelegateCall && toolCalls && (
        <div className="pl-4">
          <DelegateToolCallBadge toolCalls={toolCalls} />
        </div>
      )}
      {usage && usage.total_tokens > 0 && (
        <p className="text-[10px] text-muted-foreground/60 pl-4">
          {usage.total_tokens.toLocaleString()} tokens
          {cost != null && cost > 0 && ` · ${formatCost(cost)}`}
        </p>
      )}
    </div>
  );
}

function ToolCallList({
  toolCalls,
  className,
}: {
  toolCalls: ToolCall[];
  className?: string;
}): React.ReactNode {
  return (
    <div className={cn('space-y-1', className)}>
      {toolCalls.map((tc) => (
        <ToolCallBadge key={tc.id} toolCall={tc} />
      ))}
    </div>
  );
}

function parseToolArguments(jsonString: string): Record<string, unknown> {
  try {
    return JSON.parse(jsonString) as Record<string, unknown>;
  } catch {
    return {};
  }
}

function ToolCallBadge({ toolCall }: { toolCall: ToolCall }): React.ReactNode {
  const [expanded, setExpanded] = useState(true);
  const args = useMemo(() => parseToolArguments(toolCall.function.arguments), [toolCall.function.arguments]);

  return (
    <div className="rounded border border-border bg-muted dark:bg-surface text-xs overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-1.5 px-2 py-1.5 hover:bg-secondary transition-colors"
      >
        <Terminal className="h-3 w-3 text-muted-foreground" />
        <span className="font-mono font-medium">{toolCall.function.name}</span>
        <span className="text-muted-foreground ml-auto">{expanded ? '[-]' : '[+]'}</span>
      </button>
      {expanded && (
        <div className="px-2 py-1.5 border-t border-border bg-card dark:bg-surface">
          <ToolContentViewer toolName={toolCall.function.name} args={args} />
        </div>
      )}
    </div>
  );
}

function DelegateToolCallBadge({ toolCalls }: { toolCalls: ToolCall[] }): React.ReactNode {
  const delegateCall = toolCalls.find((tc) => tc.function.name === 'delegate');
  if (!delegateCall) return <ToolCallList toolCalls={toolCalls} />;

  const tasks = parseDelegateTasks(delegateCall);
  const nonDelegateCalls = toolCalls.filter((tc) => tc.function.name !== 'delegate');

  return (
    <div className="space-y-1">
      {nonDelegateCalls.length > 0 && <ToolCallList toolCalls={nonDelegateCalls} />}
      <div className="rounded border border-blue-500/30 bg-blue-500/5 text-xs overflow-hidden">
        <div className="flex items-center gap-1.5 px-2 py-1.5">
          <Loader2 className="h-3 w-3 text-blue-500 animate-spin" />
          <span className="font-mono font-medium">delegate</span>
          <span className="text-muted-foreground">
            {tasks.length} {tasks.length === 1 ? 'task' : 'tasks'}
          </span>
        </div>
        {tasks.length > 0 && (
          <div className="px-2 py-1 border-t border-blue-500/20 space-y-0.5">
            {tasks.map((task, i) => (
              <div key={i} className="flex items-center gap-1.5 text-muted-foreground">
                <span className="text-[10px] text-blue-500/60">{i + 1}.</span>
                <span className="truncate">{task}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function DelegateResultsInline({
  delegateIds,
  delegateStatuses,
}: {
  delegateIds: string[];
  delegateStatuses?: Record<string, DelegateInfo>;
}): React.ReactNode {
  return (
    <div className="pl-5 space-y-1">
      {delegateIds.map((id) => (
        <InlineDelegateResult
          key={id}
          delegateId={id}
          delegateInfo={delegateStatuses?.[id]}
        />
      ))}
    </div>
  );
}

function ToolResultMessage({
  toolResults,
}: {
  toolResults: ToolResult[];
}): React.ReactNode {
  return (
    <div className="pl-5 space-y-1">
      {toolResults.map((tr) => (
        <ToolResultItem key={tr.tool_use_id} result={tr} />
      ))}
    </div>
  );
}

function truncateContent(content: string, maxLength: number): string {
  if (content.length <= maxLength) {
    return content;
  }
  return content.substring(0, maxLength) + '...';
}

function ToolResultItem({ result }: { result: ToolResult }): React.ReactNode {
  const [expanded, setExpanded] = useState(false);
  const content = result.content ?? '';
  const preview = truncateContent(content, 100);

  const StatusIcon = result.is_error ? XCircle : CheckCircle;
  const statusColor = result.is_error ? 'text-red-500' : 'text-green-500';
  const borderStyle = result.is_error
    ? 'border-red-500/40 bg-red-500/10'
    : 'border-green-500/40 bg-green-500/10';

  return (
    <div className={cn('rounded border text-xs overflow-hidden', borderStyle)}>
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-1.5 px-2 py-1.5 hover:bg-accent transition-colors text-left"
      >
        <StatusIcon className={cn('h-3 w-3 flex-shrink-0', statusColor)} />
        <span className="font-mono truncate flex-1">
          {expanded ? 'Result' : preview}
        </span>
        <span className="text-muted-foreground ml-1 flex-shrink-0">
          {expanded ? '[-]' : '[+]'}
        </span>
      </button>
      {expanded && (
        <div className="px-2 py-1.5 border-t border-border bg-card dark:bg-surface">
          <pre className="text-xs overflow-x-auto whitespace-pre-wrap break-words max-h-[200px] overflow-y-auto">
            {content}
          </pre>
        </div>
      )}
    </div>
  );
}
