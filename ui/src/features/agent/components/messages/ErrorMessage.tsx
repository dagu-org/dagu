import type React from 'react';
import { XCircle } from 'lucide-react';

export function ErrorMessage({ content }: { content: string }): React.ReactNode {
  return (
    <div className="pl-1">
      <div className="flex items-start gap-1.5 text-red-500">
        <XCircle className="h-3 w-3 mt-0.5 flex-shrink-0" />
        <p className="whitespace-pre-wrap break-words">{content}</p>
      </div>
    </div>
  );
}
