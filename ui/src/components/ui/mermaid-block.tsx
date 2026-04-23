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

function isDarkMode(): boolean {
  if (typeof document === 'undefined') return false;
  return document.documentElement.classList.contains('dark');
}

function initializeMermaid(): void {
  const dark = isDarkMode();
  mermaid.initialize({
    securityLevel: 'loose',
    startOnLoad: false,
    theme: dark ? 'dark' : 'default',
    themeVariables: {
      background: 'transparent',
      primaryColor: getCSSVariable('--card', dark ? '#181b22' : '#ffffff'),
      primaryTextColor: getCSSVariable(
        '--foreground',
        dark ? '#e5e7eb' : '#111827'
      ),
      primaryBorderColor: getCSSVariable(
        '--border',
        dark ? '#303746' : '#d7dde6'
      ),
      lineColor: getCSSVariable(
        '--muted-foreground',
        dark ? '#9ca3af' : '#64748b'
      ),
      secondaryColor: getCSSVariable(
        '--secondary',
        dark ? '#242936' : '#eef2f7'
      ),
      tertiaryColor: getCSSVariable(
        '--background',
        dark ? '#111318' : '#f5f7fb'
      ),
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

  useEffect(() => {
    let cancelled = false;

    async function render() {
      if (!containerRef.current) return;

      // Generate a unique ID per render call to avoid mermaid's internal
      // diagram cache conflicts when the code or diagram type changes.
      const renderId = `mermaid-block-${idPrefix}-${mermaidIdCounter++}`;

      try {
        initializeMermaid();
        const { svg } = await mermaid.render(renderId, code.trim());
        if (!cancelled && containerRef.current) {
          containerRef.current.innerHTML = svg;
          setError(null);
        }
      } catch (err) {
        // Mermaid injects error elements into the document body on parse
        // failures. Remove them to prevent layout pollution.
        document.getElementById(renderId)?.remove();

        if (!cancelled) {
          setError(String(err));
        }
      }
    }

    render();
    return () => {
      cancelled = true;
    };
  }, [code, idPrefix]);

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
