import { cn } from '@/lib/utils';
import MonacoEditor, { loader } from '@monaco-editor/react';
import * as monaco from 'monaco-editor';
import { useEffect, useRef } from 'react';

loader.config({ monaco });

type Props = {
  value: string;
  onChange?: (value?: string) => void;
  readOnly?: boolean;
  className?: string;
};

function MarkdownEditor({ value, onChange, readOnly = false, className }: Props) {
  const editorRef = useRef<monaco.editor.IStandaloneCodeEditor | null>(null);

  useEffect(() => {
    return () => {
      editorRef.current?.dispose();
    };
  }, []);

  useEffect(() => {
    if (editorRef.current) {
      const newTheme = document.documentElement.classList.contains('dark')
        ? 'vs-dark'
        : 'vs';
      monaco.editor.setTheme(newTheme);
    }
  }, []);

  useEffect(() => {
    const observer = new MutationObserver((mutations) => {
      mutations.forEach((mutation) => {
        if (
          mutation.type === 'attributes' &&
          mutation.attributeName === 'class'
        ) {
          if (editorRef.current) {
            const newTheme = document.documentElement.classList.contains('dark')
              ? 'vs-dark'
              : 'vs';
            monaco.editor.setTheme(newTheme);
          }
        }
      });
    });

    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class'],
    });

    return () => observer.disconnect();
  }, []);

  const editorDidMount = (editor: monaco.editor.IStandaloneCodeEditor) => {
    editorRef.current = editor;
    editor.onKeyDown((e) => {
      if (e.code === 'KeyF' && !e.ctrlKey && !e.metaKey && !e.altKey) {
        e.stopPropagation();
      }
    });
  };

  const isDarkMode =
    typeof window !== 'undefined' &&
    document.documentElement.classList.contains('dark');

  return (
    <div className={cn('h-full', className)}>
      <MonacoEditor
        height="100%"
        language="markdown"
        theme={isDarkMode ? 'vs-dark' : 'vs'}
        value={value}
        onChange={readOnly ? undefined : onChange}
        onMount={editorDidMount}
        options={{
          readOnly,
          minimap: { enabled: false },
          scrollBeyondLastLine: false,
          wordWrap: 'on',
          lineNumbers: 'on',
          glyphMargin: false,
          fontFamily:
            "'JetBrains Mono', 'Fira Code', Menlo, Monaco, 'Courier New', monospace",
          fontSize: 13,
          padding: { top: 8, bottom: 8 },
          quickSuggestions: false,
          renderValidationDecorations: 'off',
        }}
      />
    </div>
  );
}

export default MarkdownEditor;
