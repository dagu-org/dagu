// Agent chat types

export type MessageType = 'user' | 'assistant' | 'tool_use' | 'tool_result' | 'error' | 'ui_action';

export interface UIAction {
  type: 'navigate' | 'refresh';
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

export interface StreamResponse {
  messages?: Message[];
  conversation?: Conversation;
  conversation_state?: ConversationState;
}

export interface ConversationWithState {
  conversation: Conversation;
  working: boolean;
  model?: string;
}

export interface NewConversationResponse {
  conversation_id: string;
  status: string;
}

export interface ChatRequest {
  message: string;
  model?: string;
}
