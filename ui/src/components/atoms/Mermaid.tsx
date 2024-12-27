import React, { CSSProperties } from 'react';
import mermaid from 'mermaid';
import '@fortawesome/fontawesome-free/css/all.min.css';

type Props = {
  def: string;
  style?: CSSProperties;
};

// Mermaidの初期設定
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

function Mermaid({ def, style = {} }: Props) {
  const ref = React.useRef<HTMLDivElement>(null);
  const [uniqueId] = React.useState(
    () => `mermaid-${Math.random().toString(36).substr(2, 9)}`
  );

  const mStyle = {
    ...style,
  };

  const dStyle: CSSProperties = {
    overflowX: 'auto',
    padding: '2em',
  };

  const render = async () => {
    if (!ref.current) {
      return;
    }
    if (def.startsWith('<')) {
      console.error('invalid definition!!');
      return;
    }

    try {
      // Clear previous content
      ref.current.innerHTML = '';

      // Generate SVG
      const { svg, bindFunctions } = await mermaid.render(uniqueId, def);

      if (ref.current) {
        ref.current.innerHTML = svg;

        // Bind event handlers
        setTimeout(() => {
          if (ref.current && bindFunctions) {
            bindFunctions(ref.current);
          }
        }, 500);
      }
    } catch (error: unknown) {
      console.error('Mermaid render error:', error);
      if (ref.current) {
        ref.current.innerHTML = `
          <div style="color: red; padding: 10px;">
            Error rendering diagram: ${error}
          </div>
        `;
      }
    }
  };

  const renderWithRetry = () => {
    try {
      render();
    } catch (error) {
      console.error('error rendering mermaid, retrying, error:');
      console.error(error);
      console.error(def);
      setTimeout(renderWithRetry, 1);
    }
  };

  React.useEffect(() => {
    renderWithRetry();
  }, [def]);

  return (
    <div style={dStyle}>
      <div className="mermaid" ref={ref} style={mStyle} />
    </div>
  );
}

// メモ化の条件を維持
export default React.memo(Mermaid, (prev, next) => {
  return prev.def === next.def;
});
