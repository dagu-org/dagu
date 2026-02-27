import mermaid from 'mermaid';
import { useEffect, useId, useRef, useState } from 'react';

// Helper function to get computed CSS variable value with fallback
function getCSSVariable(name: string, fallback: string): string {
  if (typeof window === 'undefined') return fallback;
  const value = getComputedStyle(document.documentElement)
    .getPropertyValue(name)
    .trim();
  return value || fallback;
}

function initializeMermaid(): void {
  mermaid.initialize({
    securityLevel: 'strict',
    startOnLoad: false,
    theme: 'default',
    themeVariables: {
      background: 'transparent',
      primaryColor: getCSSVariable('--card', '#faf8f5'),
      primaryTextColor: getCSSVariable('--foreground', '#3d3833'),
      primaryBorderColor: getCSSVariable('--border', '#c8bfb0'),
      lineColor: getCSSVariable('--muted-foreground', '#6b635a'),
      secondaryColor: getCSSVariable('--secondary', '#f0ebe3'),
      tertiaryColor: getCSSVariable('--background', '#f5f0e8'),
    },
    fontFamily: 'Arial',
    logLevel: 4,
  });
}

let mermaidIdCounter = 0;

interface MermaidBlockProps {
  code: string;
}

export function MermaidBlock({ code }: MermaidBlockProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [error, setError] = useState<string | null>(null);
  const idPrefix = useId().replace(/:/g, '');
  const renderIdRef = useRef(`mermaid-block-${idPrefix}-${mermaidIdCounter++}`);

  useEffect(() => {
    let cancelled = false;

    async function render() {
      if (!containerRef.current) return;

      try {
        initializeMermaid();
        const { svg } = await mermaid.render(renderIdRef.current, code.trim());
        if (!cancelled && containerRef.current) {
          containerRef.current.innerHTML = svg;
          setError(null);
        }
      } catch (err) {
        if (!cancelled) {
          setError(String(err));
        }
      }
    }

    render();
    return () => { cancelled = true; };
  }, [code]);

  if (error) {
    return (
      <pre className="text-xs p-2 rounded bg-muted overflow-x-auto font-mono">
        <code>{code}</code>
      </pre>
    );
  }

  return (
    <div
      ref={containerRef}
      className="my-1 overflow-x-auto [&>svg]:max-w-full"
    />
  );
}
