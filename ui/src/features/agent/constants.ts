export const ANIMATION_CLOSE_DURATION_MS = 250;
export const ANIMATION_OPEN_DURATION_MS = 400;
export const DELEGATE_PANEL_BASE_Z_INDEX = 60;
export const TASK_TRUNCATE_LENGTH = 50;

export function truncateTask(task: string): string {
  return task.length > TASK_TRUNCATE_LENGTH ? task.slice(0, TASK_TRUNCATE_LENGTH) + '...' : task;
}
export const TOOL_RESULT_PREVIEW_LENGTH = 100;
export const MAX_SSE_RETRIES = 3;
export const DELEGATE_PANEL_WIDTH = 320;
export const DELEGATE_PANEL_HEIGHT = 360;
export const DELEGATE_PANEL_GAP = 12;
export const DELEGATE_PANEL_MARGIN = 16;
export const DELEGATE_PANEL_MIN_WIDTH = 280;
export const DELEGATE_PANEL_MIN_HEIGHT = 200;
