import { ChevronRight } from 'lucide-react';
import { cn } from '@/lib/utils';

export function UserMessage({ content, isPending }: { content: string; isPending?: boolean }): React.ReactNode {
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
