import { useOptionalPageContext } from '@/contexts/PageContext';
import type { DocRef } from '../components/DocPicker';

/**
 * Returns the current page's doc context for agent chat, or null if not viewing a doc.
 */
export function useDocPageContext(): DocRef | null {
  const context = useOptionalPageContext()?.context;

  if (!context?.docPath) {
    return null;
  }

  return {
    id: context.docPath,
    title: context.docTitle || context.docPath,
  };
}
