import React, { CSSProperties } from 'react';
import mermaid from 'mermaid';
import '@fortawesome/fontawesome-free/css/all.min.css';
import { IconButton, Stack } from '@mui/material';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import {
  faMagnifyingGlassPlus,
  faMagnifyingGlassMinus,
  faRotateLeft,
} from '@fortawesome/free-solid-svg-icons';

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
  const [scale, setScale] = React.useState(1);

  const mStyle = {
    ...style,
  };

  const dStyle: CSSProperties = {
    overflowX: 'auto',
    padding: '2em',
  };

  const zoomIn = () => {
    setScale((prevScale) => Math.min(prevScale + 0.1, 2));
  };

  const zoomOut = () => {
    setScale((prevScale) => Math.max(prevScale - 0.1, 0.5));
  };

  const resetZoom = () => {
    setScale(1);
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

  React.useEffect(() => {
    if (ref.current) {
      const svg = ref.current.querySelector('svg');
      if (svg) {
        svg.style.transform = `scale(${scale})`;
        svg.style.transformOrigin = 'top left';
      }
    }
  }, [scale]);

  return (
    <div style={dStyle}>
      <Stack direction="row" justifyContent="right" spacing={1} sx={{ mb: 2 }}>
        <IconButton
          size="small"
          onClick={zoomIn}
          sx={{
            border: '1px solid rgba(0, 0, 0, 0.12)',
            borderRadius: '4px',
            color: 'rgba(0, 0, 0, 0.54)',
            padding: '8px',
            '&:hover': {
              backgroundColor: 'rgba(0, 0, 0, 0.04)',
            },
          }}
        >
          <FontAwesomeIcon icon={faMagnifyingGlassPlus} />
        </IconButton>
        <IconButton
          size="small"
          onClick={zoomOut}
          sx={{
            border: '1px solid rgba(0, 0, 0, 0.12)',
            borderRadius: '4px',
            color: 'rgba(0, 0, 0, 0.54)',
            padding: '8px',
            '&:hover': {
              backgroundColor: 'rgba(0, 0, 0, 0.04)',
            },
          }}
        >
          <FontAwesomeIcon icon={faMagnifyingGlassMinus} />
        </IconButton>
        <IconButton
          size="small"
          onClick={resetZoom}
          sx={{
            border: '1px solid rgba(0, 0, 0, 0.12)',
            borderRadius: '4px',
            color: 'rgba(0, 0, 0, 0.54)',
            padding: '8px',
            '&:hover': {
              backgroundColor: 'rgba(0, 0, 0, 0.04)',
            },
          }}
        >
          <FontAwesomeIcon icon={faRotateLeft} />
        </IconButton>
      </Stack>
      <div
        className="mermaid"
        ref={ref}
        style={{
          ...mStyle,
          overflow: 'auto',
          maxHeight: '80vh',
        }}
      />
    </div>
  );
}

// メモ化の条件を維持
export default React.memo(Mermaid, (prev, next) => {
  return prev.def === next.def;
});
