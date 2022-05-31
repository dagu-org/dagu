import React, { CSSProperties } from "react";
import mermaidAPI from "mermaid";

type Props = {
  def: string;
  style?: CSSProperties;
};

mermaidAPI.initialize({
  // @ts-ignore
  securityLevel: "loose",
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
    overflowX: "auto",
  };
  function render() {
    if (!ref.current) {
      return;
    }
    console.log({
      def,
    });
    if (def.startsWith("<")) {
      console.error("invalid definition!!");
      return;
    }
    mermaidAPI.render(
      "mermaid",
      def,
      (svgCode, bindFunc) => {
        console.log({ svgCode });
        if (ref.current) {
          // @ts-ignore
          ref.current.innerHTML = svgCode;
        }
        setTimeout(() => {
          if (ref.current) {
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
      console.error("error rendering mermaid, retrying, error:");
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
