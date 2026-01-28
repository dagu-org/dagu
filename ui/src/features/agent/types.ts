// Message types
export type MessageType =
  | 'user'
  | 'assistant'
  | 'tool_use'
  | 'tool_result'
  | 'error'
  | 'ui_action';

export type UIActionType = 'navigate' | 'refresh';

export interface UIAction {
  type: UIActionType;
  path?: string;
}

export interface ToolCall {
  id: string;
  type: string;
  function: {
    name: string;
    arguments: string;
  };
}

export interface ToolResult {
  tool_use_id: string;
  content: string;
  is_error?: boolean;
}

export interface Message {
  id: string;
  conversation_id: string;
  type: MessageType;
  sequence_id: number;
  content?: string;
  tool_calls?: ToolCall[];
  tool_results?: ToolResult[];
  ui_action?: UIAction;
  created_at: string;
}

// Conversation types
export interface Conversation {
  id: string;
  created_at: string;
  updated_at: string;
}

export interface ConversationState {
  conversation_id: string;
  working: boolean;
  model?: string;
}

export interface ConversationWithState {
  conversation: Conversation;
  working: boolean;
  model?: string;
}

// DAG context types
export interface DAGContext {
  dag_file: string;
  dag_run_id?: string;
}

// API request/response types
export interface ChatRequest {
  message: string;
  model?: string;
  dag_contexts?: DAGContext[];
}

export interface NewConversationResponse {
  conversation_id: string;
  status: string;
}

export interface StreamResponse {
  messages?: Message[];
  conversation?: Conversation;
  conversation_state?: ConversationState;
}
