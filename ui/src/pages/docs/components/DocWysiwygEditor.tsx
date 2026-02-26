import { Crepe, CrepeFeature } from '@milkdown/crepe';
import { Milkdown, MilkdownProvider, useEditor } from '@milkdown/react';
import { useEffect, useRef, useState } from 'react';

import '@milkdown/crepe/theme/common/style.css';
import '@milkdown/crepe/theme/frame.css';
import './doc-wysiwyg.css';

interface DocWysiwygEditorProps {
  value: string;
  onChange?: (value: string) => void;
  readOnly?: boolean;
}

function MilkdownEditor({ value, onChange, readOnly }: DocWysiwygEditorProps) {
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  const initializedRef = useRef(false);
  const crepeRef = useRef<Crepe | null>(null);

  useEditor((root) => {
    const crepe = new Crepe({
      root,
      defaultValue: value,
      features: {
        [CrepeFeature.CodeMirror]: false,
        [CrepeFeature.Latex]: false,
        [CrepeFeature.ImageBlock]: false,
      },
    });

    crepe.on((listener) => {
      listener.markdownUpdated((_ctx, markdown, prevMarkdown) => {
        if (!initializedRef.current) {
          initializedRef.current = true;
          return;
        }
        if (markdown !== prevMarkdown) {
          onChangeRef.current?.(markdown);
        }
      });
    });

    if (readOnly) {
      crepe.setReadonly(true);
    }

    crepeRef.current = crepe;
    return crepe;
  }, []);

  useEffect(() => {
    if (crepeRef.current) {
      crepeRef.current.setReadonly(!!readOnly);
    }
  }, [readOnly]);

  return <Milkdown />;
}

export function DocWysiwygEditor({ value, onChange, readOnly }: DocWysiwygEditorProps) {
  const [darkKey, setDarkKey] = useState(
    document.documentElement.classList.contains('dark') ? 'dark' : 'light'
  );

  useEffect(() => {
    const observer = new MutationObserver(() => {
      const next = document.documentElement.classList.contains('dark') ? 'dark' : 'light';
      setDarkKey(next);
    });
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class'],
    });
    return () => observer.disconnect();
  }, []);

  return (
    <div className="doc-wysiwyg-container h-full">
      <MilkdownProvider key={darkKey}>
        <MilkdownEditor value={value} onChange={onChange} readOnly={readOnly} />
      </MilkdownProvider>
    </div>
  );
}
