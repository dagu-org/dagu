import { cn } from '@/lib/utils';
import { slugifyHeading } from '@/lib/text-utils';
import { ChevronDown, ChevronRight, List } from 'lucide-react';
import React, { useMemo, useState } from 'react';

export interface OutlineHeading {
  level: number;
  text: string;
  anchor: string;
}

export function extractHeadings(markdown: string | null | undefined): OutlineHeading[] {
  if (!markdown) return [];
  const headings: OutlineHeading[] = [];
  const lines = markdown.split('\n');
  let inCodeBlock = false;

  for (const line of lines) {
    if (line.trimStart().startsWith('```')) {
      inCodeBlock = !inCodeBlock;
      continue;
    }
    if (inCodeBlock) continue;

    const match = line.match(/^(#{1,6})\s+(.+)$/);
    if (match && match[1] && match[2]) {
      const level = match[1].length;
      const text = match[2].trim();
      headings.push({ level, text, anchor: slugifyHeading(text) });
    }
  }
  return headings;
}

type Props = {
  markdown: string | null | undefined;
  onHeadingClick: (anchor: string) => void;
};

function DocOutlinePanel({ markdown, onHeadingClick }: Props) {
  const [collapsed, setCollapsed] = useState(false);
  const headings = useMemo(() => extractHeadings(markdown), [markdown]);

  if (headings.length === 0) return null;

  return (
    <div className="border-t border-border">
      <button
        type="button"
        className="flex items-center gap-1.5 w-full px-3 py-1.5 text-xs font-medium uppercase tracking-wide text-muted-foreground hover:text-foreground"
        onClick={() => setCollapsed((c) => !c)}
      >
        {collapsed ? (
          <ChevronRight className="h-3 w-3" />
        ) : (
          <ChevronDown className="h-3 w-3" />
        )}
        <List className="h-3 w-3" />
        Outline
      </button>
      {!collapsed && (
        <div className="overflow-y-auto max-h-48 pb-1">
          {headings.map((h, i) => (
            <button
              key={`${h.anchor}-${i}`}
              type="button"
              className={cn(
                'block w-full text-left text-xs truncate py-0.5 px-3 hover:bg-accent/50 text-muted-foreground hover:text-foreground'
              )}
              style={{ paddingLeft: `${(h.level - 1) * 12 + 12}px` }}
              onClick={() => onHeadingClick(h.anchor)}
              title={h.text}
            >
              {h.text}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

export default DocOutlinePanel;
