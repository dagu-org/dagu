import React, { CSSProperties } from "react";
import mermaidAPI from "mermaid";
import { Box } from "@mui/material";

type Props = {
  children: string;
  style?: CSSProperties;
};

function Mermaid({ children, style = {} }: Props) {
  const [html, setHtml] = React.useState("");
  const divRef = React.useRef<HTMLDivElement>(null);
  const mStyle = {
    ...style,
  };
  const dStyle: CSSProperties = {
    overflowX: "auto",
  };
  React.useEffect(() => {
    if (!divRef.current) {
      return;
    }
    try {
      mermaidAPI.initialize({
        // @ts-ignore
        securityLevel: "loose",
        startOnLoad: true,
        maxTextSize: 99999999,
        flowchart: {
          useMaxWidth: false,
          htmlLabels: true,
        },
      });
      mermaidAPI.render(
        "mermaid",
        children,
        (svgCode, bindFunc) => {
          setHtml(svgCode);
          setTimeout(() => {
            if (divRef.current) {
              bindFunc(divRef.current);
            }
          }, 500);
        },
        divRef.current
      );
    } catch (error) {
      console.error(error);
      console.error(children);
    }
  }, [children, divRef]);
  const param = { __html: html };
  return (
    <Box sx={dStyle}>
      <Box
        className="mermaid"
        dangerouslySetInnerHTML={param}
        ref={divRef}
        sx={mStyle}
      />
    </Box>
  );
}

export default Mermaid;
