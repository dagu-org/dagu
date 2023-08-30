import React, { CSSProperties } from 'react';
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
import mermaidAPI, { Mermaid } from 'mermaid';

type Props = {
  def: string;
  style?: CSSProperties;
};

declare global {
  interface Mermaid {
    securityLevel: string;
  }
}

mermaidAPI.initialize({
  securityLevel: 'loose' as Mermaid['mermaidAPI']['SecurityLevel']['Loose'],
  startOnLoad: false,
  maxTextSize: 99999999,
  flowchart: {
    useMaxWidth: false,
    htmlLabels: true,
  },
  logLevel: 4, // ERROR
});

function Mermaid({ def, style = {} }: Props) {
  const ref = React.useRef<HTMLDivElement>(null);
  const mStyle = {
    ...style,
  };
  const dStyle: CSSProperties = {
    overflowX: 'auto',
    padding: '2em',
  };
  function render() {
    if (!ref.current) {
      return;
    }
    if (def.startsWith('<')) {
      console.error('invalid definition!!');
      return;
    }
    mermaidAPI.render(
      'mermaid',
      def,
      (svgCode, bindFunc) => {
        if (ref.current) {
          ref.current.innerHTML = svgCode;
        }
        setTimeout(() => {
          if (ref.current) {
            // eslint-disable-next-line @typescript-eslint/ban-ts-comment
            // @ts-ignore
            bindFunc(ref.current);
          }
        }, 500);
      },
      ref.current
    );
  }
  function renderWithRetry() {
    try {
      render();
    } catch (error) {
      console.error('error rendering mermaid, retrying, error:');
      console.error(error);
      console.error(def);
      setTimeout(renderWithRetry, 1);
    }
  }
  React.useEffect(() => {
    renderWithRetry();
  }, [def, ref.current]);
  return (
    <div style={dStyle}>
      <div className="mermaid" ref={ref} style={mStyle} />
    </div>
  );
}

export default React.memo(Mermaid, (prev, next) => {
  return prev.def == next.def;
});
