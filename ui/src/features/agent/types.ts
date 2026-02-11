// Message types
export type MessageType =
  | 'user'
  | 'assistant'
  | 'tool_use'
  | 'tool_result'
  | 'error'
  | 'ui_action'
  | 'user_prompt';

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

export interface UserPromptOption {
  id: string;
  label: string;
  description?: string;
}

export type PromptType = 'general' | 'command_approval';

export interface UserPrompt {
  prompt_id: string;
  question: string;
  options?: UserPromptOption[];
  allow_free_text: boolean;
  free_text_placeholder?: string;
  multi_select: boolean;
  prompt_type?: PromptType;
  command?: string;
  working_dir?: string;
}

export interface UserPromptResponse {
  prompt_id: string;
  selected_option_ids?: string[];
  free_text_response?: string;
  cancelled?: boolean;
}

export interface TokenUsage {
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
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
  user_prompt?: UserPrompt;
  usage?: TokenUsage;
  cost?: number;
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
  total_cost?: number;
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
  safe_mode?: boolean;
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

// Tool input types for specialized viewers
export interface BashToolInput {
  command: string;
  timeout?: number;
}

export interface ReadToolInput {
  path: string;
  offset?: number;
  limit?: number;
}

export interface PatchToolInput {
  path: string;
  operation?: 'create' | 'replace' | 'delete';
  content?: string;
  old_string?: string;
  new_string?: string;
}

export interface ThinkToolInput {
  thought: string;
}

export interface NavigateToolInput {
  path: string;
}

export interface ReadSchemaToolInput {
  schema: string;
  path?: string;
}

export interface AskUserToolInput {
  question: string;
  options?: UserPromptOption[];
  allow_free_text?: boolean;
  free_text_placeholder?: string;
  multi_select?: boolean;
}
