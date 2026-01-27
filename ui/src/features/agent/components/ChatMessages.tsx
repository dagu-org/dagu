import * as React from 'react';
import { useEffect, useRef } from 'react';
import { ChevronRight, Terminal, CheckCircle, XCircle, Loader2, ExternalLink } from 'lucide-react';
import { Message, ToolCall, ToolResult, UIAction } from '../types';
import { cn } from '@/lib/utils';

interface ChatMessagesProps {
  messages: Message[];
  isWorking: boolean;
}

export function ChatMessages({ messages, isWorking }: ChatMessagesProps) {
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom when messages change
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  if (messages.length === 0) {
    return (
      <div className="flex-1 flex items-center justify-center text-muted-foreground p-4">
        <div className="text-center">
          <Terminal className="h-8 w-8 mx-auto mb-2 opacity-30" />
          <p className="text-xs text-muted-foreground">
            Ask the agent to create DAGs, run commands, or help with workflows
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex-1 overflow-y-auto p-2 space-y-2 font-mono text-xs">
      {messages.map((message) => (
        <MessageItem key={message.id} message={message} />
      ))}
      {isWorking && (
        <div className="flex items-center gap-1.5 text-yellow-500 pl-1">
          <Loader2 className="h-3 w-3 animate-spin" />
          <span>processing...</span>
        </div>
      )}
      <div ref={messagesEndRef} />
    </div>
  );
}

function MessageItem({ message }: { message: Message }) {
  switch (message.type) {
    case 'user':
      return <UserMessage content={message.content || ''} />;
    case 'assistant':
      return (
        <AssistantMessage
          content={message.content || ''}
          toolCalls={message.tool_calls}
        />
      );
    case 'tool_use':
      return <ToolUseMessage toolCalls={message.tool_calls || []} />;
    case 'tool_result':
      return <ToolResultMessage toolResults={message.tool_results || []} />;
    case 'error':
      return <ErrorMessage content={message.content || ''} />;
    case 'ui_action':
      return <UIActionMessage action={message.ui_action} />;
    default:
      return null;
  }
}

function ErrorMessage({ content }: { content: string }) {
  return (
    <div className="pl-1">
      <div className="flex items-start gap-1.5 text-red-500">
        <XCircle className="h-3 w-3 mt-0.5 flex-shrink-0" />
        <p className="whitespace-pre-wrap break-words">{content}</p>
      </div>
    </div>
  );
}

function UIActionMessage({ action }: { action?: UIAction }) {
  if (!action) return null;

  if (action.type === 'navigate') {
    return (
      <div className="pl-1">
        <div className="flex items-center gap-1.5 text-muted-foreground">
          <ExternalLink className="h-3 w-3 flex-shrink-0" />
          <span>Navigating to {action.path}</span>
        </div>
      </div>
    );
  }

  return null;
}

function UserMessage({ content }: { content: string }) {
  return (
    <div className="pl-1">
      <div className="flex items-start gap-1.5 text-primary">
        <ChevronRight className="h-3 w-3 mt-0.5 flex-shrink-0" />
        <p className="whitespace-pre-wrap break-words">{content}</p>
      </div>
    </div>
  );
}

function AssistantMessage({
  content,
  toolCalls,
}: {
  content: string;
  toolCalls?: ToolCall[];
}) {
  return (
    <div className="pl-1 space-y-1">
      {content && (
        <p className="whitespace-pre-wrap break-words text-foreground/90 pl-4">{content}</p>
      )}
      {toolCalls && toolCalls.length > 0 && (
        <div className="space-y-1 pl-4">
          {toolCalls.map((tc) => (
            <ToolCallBadge key={tc.id} toolCall={tc} />
          ))}
        </div>
      )}
    </div>
  );
}

function ToolUseMessage({ toolCalls }: { toolCalls: ToolCall[] }) {
  return (
    <div className="pl-5 space-y-1">
      {toolCalls.map((tc) => (
        <ToolCallBadge key={tc.id} toolCall={tc} />
      ))}
    </div>
  );
}

function ToolCallBadge({ toolCall }: { toolCall: ToolCall }) {
  const [expanded, setExpanded] = React.useState(false);

  let args: Record<string, unknown> = {};
  try {
    args = JSON.parse(toolCall.function.arguments);
  } catch {
    // Keep empty object
  }

  return (
    <div className="rounded border border-border/60 bg-muted/50 text-xs overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-1.5 px-2 py-1.5 hover:bg-muted/80 transition-colors"
      >
        <Terminal className="h-3 w-3 text-muted-foreground" />
        <span className="font-mono font-medium">{toolCall.function.name}</span>
        <span className="text-muted-foreground ml-auto">
          {expanded ? '[-]' : '[+]'}
        </span>
      </button>
      {expanded && (
        <div className="px-2 py-1.5 border-t border-border/40 bg-background/50">
          <pre className="text-xs overflow-x-auto whitespace-pre-wrap break-words">
            {JSON.stringify(args, null, 2)}
          </pre>
        </div>
      )}
    </div>
  );
}

function ToolResultMessage({ toolResults }: { toolResults: ToolResult[] }) {
  return (
    <div className="pl-5 space-y-1">
      {toolResults.map((tr) => (
        <ToolResultItem key={tr.tool_use_id} result={tr} />
      ))}
    </div>
  );
}

function ToolResultItem({ result }: { result: ToolResult }) {
  const [expanded, setExpanded] = React.useState(false);
  const isError = result.is_error;
  const content = result.content || '';
  const preview =
    content.length > 100 ? content.substring(0, 100) + '...' : content;

  return (
    <div
      className={cn(
        'rounded border text-xs overflow-hidden',
        isError
          ? 'border-red-500/40 bg-red-500/10'
          : 'border-green-500/40 bg-green-500/10'
      )}
    >
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-1.5 px-2 py-1.5 hover:bg-muted/30 transition-colors text-left"
      >
        {isError ? (
          <XCircle className="h-3 w-3 text-red-500 flex-shrink-0" />
        ) : (
          <CheckCircle className="h-3 w-3 text-green-500 flex-shrink-0" />
        )}
        <span className="font-mono truncate flex-1">
          {expanded ? 'Result' : preview}
        </span>
        <span className="text-muted-foreground ml-1 flex-shrink-0">
          {expanded ? '[-]' : '[+]'}
        </span>
      </button>
      {expanded && (
        <div className="px-2 py-1.5 border-t border-border/40 bg-background/50">
          <pre className="text-xs overflow-x-auto whitespace-pre-wrap break-words max-h-[200px] overflow-y-auto">
            {content}
          </pre>
        </div>
      )}
    </div>
  );
}
