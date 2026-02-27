import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { cn } from '@/lib/utils';
import { MermaidBlock } from '@/components/ui/mermaid-block';

interface MarkdownProps {
  content: string;
  className?: string;
}

// Compact markdown renderer for chat messages
export function Markdown({ content, className }: MarkdownProps) {
  return (
    <div
      className={cn(
        'text-xs prose prose-sm prose-slate dark:prose-invert max-w-none break-words',
        // Compact spacing
        'prose-p:my-1 prose-p:leading-relaxed',
        'prose-pre:my-1 prose-pre:p-2 prose-pre:text-xs prose-pre:bg-muted prose-pre:rounded',
        'prose-code:text-xs prose-code:bg-muted prose-code:px-1 prose-code:py-0.5 prose-code:rounded prose-code:before:content-none prose-code:after:content-none',
        'prose-ul:my-1 prose-ul:pl-4',
        'prose-ol:my-1 prose-ol:pl-4',
        'prose-li:my-0.5',
        'prose-headings:my-1 prose-headings:font-semibold',
        'prose-h1:text-sm prose-h2:text-sm prose-h3:text-xs',
        'prose-blockquote:my-1 prose-blockquote:pl-2 prose-blockquote:border-l-2',
        'prose-table:my-1 prose-table:text-xs',
        'prose-a:text-blue-600 dark:prose-a:text-blue-400',
        className
      )}
    >
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          // Custom code block rendering
          code({ className: codeClassName, children, ...props }) {
            const isInline = !codeClassName;
            if (isInline) {
              return (
                <code
                  className="text-xs bg-muted px-1 py-0.5 rounded font-mono"
                  {...props}
                >
                  {children}
                </code>
              );
            }
            // Mermaid code blocks
            if (codeClassName === 'language-mermaid') {
              return <MermaidBlock code={String(children)} />;
            }
            // Block code
            return (
              <code className={cn('block text-xs font-mono', codeClassName)} {...props}>
                {children}
              </code>
            );
          },
          // Ensure pre blocks have proper styling
          pre({ children, ...props }) {
            // Pass through mermaid blocks without wrapping in pre
            const child = children as React.ReactElement;
            if (child?.type === MermaidBlock) {
              return <>{children}</>;
            }
            return (
              <pre
                className="text-xs p-2 rounded bg-muted overflow-x-auto font-mono"
                {...props}
              >
                {children}
              </pre>
            );
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
