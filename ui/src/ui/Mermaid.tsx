import mermaid from 'mermaid';
import React, { CSSProperties } from 'react';

type Props = {
  def: string;
  style?: CSSProperties;
  scale: number;
};

mermaid.initialize({
  securityLevel: 'loose',
  startOnLoad: false,
  maxTextSize: 99999999,
  flowchart: {
    curve: 'basis',
    useMaxWidth: false,
    htmlLabels: true,
    nodeSpacing: 50,
    rankSpacing: 50,
  },
  fontFamily: 'Arial',
  logLevel: 4, // ERROR
});

function Mermaid({ def, style = {}, scale }: Props) {
  const mermaidRef = React.useRef<HTMLDivElement>(null); // Ref for the inner div holding the SVG
  const scrollContainerRef = React.useRef<HTMLDivElement>(null); // Ref for the outer scrollable div
  const scrollPosRef = React.useRef({ top: 0, left: 0 }); // Ref to store scroll position
  const [uniqueId] = React.useState(
    () => `mermaid-${Math.random().toString(36).substr(2, 9)}`
  );

  const mStyle = {
    ...style,
  };

  const dStyle: CSSProperties = {
    overflow: 'auto', // Use 'auto' for both directions if needed
    padding: '2em',
    position: 'relative',
    maxHeight: '80vh', // Keep max height if desired
  };

  const render = async () => {
    if (!mermaidRef.current) {
      return;
    }
    if (def.startsWith('<')) {
      console.error('invalid definition!!');
      return;
    }

    try {
      // Clear previous content
      mermaidRef.current.innerHTML = '';

      // Generate SVG
      const { svg, bindFunctions } = await mermaid.render(uniqueId, def);

      if (mermaidRef.current) {
        mermaidRef.current.innerHTML = svg;

        // Apply scale transform immediately after SVG is rendered
        const svgEl = mermaidRef.current.querySelector('svg');
        if (svgEl) {
          svgEl.style.overflow = 'visible';
          svgEl.style.transform = `scale(${scale})`;
          svgEl.style.transformOrigin = 'top left';
        }

        // Restore scroll position *after* SVG is rendered
        if (scrollContainerRef.current) {
          scrollContainerRef.current.scrollTop = scrollPosRef.current.top;
          scrollContainerRef.current.scrollLeft = scrollPosRef.current.left;
        }

        // Bind event handlers
        // Consider if this timeout is still necessary or can be reduced
        setTimeout(() => {
          if (mermaidRef.current && bindFunctions) {
            bindFunctions(mermaidRef.current);
          }
        }, 100); // Reduced timeout slightly
      }
    } catch (error: unknown) {
      console.error('Mermaid render error:', error);
      if (mermaidRef.current) {
        mermaidRef.current.innerHTML = `
          <div style="color: red; padding: 10px; white-space: pre-wrap;">
            Error rendering diagram: ${String(error)}
          </div>
        `;
      }
    }
  };

  React.useEffect(() => {
    // Save scroll position before re-rendering
    if (scrollContainerRef.current) {
      scrollPosRef.current = {
        top: scrollContainerRef.current.scrollTop,
        left: scrollContainerRef.current.scrollLeft,
      };
    }
    render();
  }, [def]); // Only trigger re-render on definition change

  React.useEffect(() => {
    // Apply scale transformation when scale prop changes
    if (mermaidRef.current) {
      const svg = mermaidRef.current.querySelector('svg');
      if (svg) {
        // Ensure the SVG itself doesn't cause overflow issues conflicting with the container
        svg.style.overflow = 'visible';
        svg.style.transform = `scale(${scale})`;
        svg.style.transformOrigin = 'top left'; // Keep origin consistent
      }
    }
  }, [scale]); // Apply scale separately

  return (
    // Attach ref to the scrollable container
    <div ref={scrollContainerRef} style={dStyle}>
      <div
        className="mermaid"
        ref={mermaidRef} // Keep ref for mermaid rendering target
        style={{
          ...mStyle,
          // Remove overflow from inner div, let outer div handle it
          // overflow: 'auto',
          // maxHeight: '80vh', // Max height is now on the outer div
        }}
      />
    </div>
  );
}

export default React.memo(Mermaid, (prev, next) => {
  return prev.def === next.def && prev.scale === next.scale;
});
