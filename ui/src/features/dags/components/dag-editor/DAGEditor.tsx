/**
 * DAGEditor component provides a Monaco editor for editing DAG YAML definitions.
 *
 * @module features/dags/components/dag-editor
 */
import { cn } from '@/lib/utils';
import MonacoEditor, { loader } from '@monaco-editor/react';
import * as monaco from 'monaco-editor';
import { configureMonacoYaml } from 'monaco-yaml';
import { useEffect, useRef } from 'react';

// Configure schema at module level (before editor initialization)
configureMonacoYaml(monaco, {
  enableSchemaRequest: true,
  hover: true,
  completion: true,
  validate: true,
  format: true,
  schemas: [
    {
      uri: 'https://raw.githubusercontent.com/dagu-org/dagu/main/schemas/dag.schema.json',
      fileMatch: ['*'], // Match all YAML files
    },
  ],
});

loader.config({ monaco });

/**
 * Cursor position information
 */
export interface CursorPosition {
  lineNumber: number;
  column: number;
}

/**
 * Props for the DAGEditor component
 */
type Props = {
  /** Current YAML content */
  value: string;
  /** Callback function when content changes */
  onChange?: (value?: string) => void;
  /** Whether the editor is in read-only mode */
  readOnly?: boolean;
  /** Whether to show line numbers */
  lineNumbers?: boolean;
  /** Line number to highlight */
  highlightLine?: number;
  /** Additional class name */
  className?: string;
  /** Callback when cursor position changes */
  onCursorPositionChange?: (position: CursorPosition) => void;
};

/**
 * DAGEditor component provides a Monaco editor with YAML schema validation
 * for editing or viewing DAG definitions
 */
function DAGEditor({
  value,
  onChange,
  readOnly = false,
  lineNumbers = true,
  className,
  onCursorPositionChange,
}: Omit<Props, 'highlightLine'>) {
  const editorRef = useRef<monaco.editor.IStandaloneCodeEditor | null>(null);

  // Clean up editor on unmount
  useEffect(() => {
    return () => {
      editorRef.current?.dispose();
    };
  }, []);

  // Update editor theme when dark mode changes
  useEffect(() => {
    if (editorRef.current) {
      const newTheme = document.documentElement.classList.contains('dark')
        ? 'vs-dark'
        : 'vs';
      monaco.editor.setTheme(newTheme);
    }
  }, []);

  // Listen for theme changes
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

  /**
   * Initialize editor after mounting
   */
  const editorDidMount = (editor: monaco.editor.IStandaloneCodeEditor) => {
    editorRef.current = editor;

    // Format document after a short delay
    setTimeout(() => {
      editor.getAction('editor.action.formatDocument')?.run();
    }, 100);

    // Prevent 'f' key from propagating to prevent fullscreen shortcuts
    // when user is typing in the editor
    editor.onKeyDown((e) => {
      if (e.code === 'KeyF' && !e.ctrlKey && !e.metaKey && !e.altKey) {
        // Stop the 'f' key from propagating to parent components
        // that might have fullscreen shortcuts
        e.stopPropagation();
      }
    });

    // Listen for cursor position changes
    if (onCursorPositionChange) {
      // Initial position
      const position = editor.getPosition();
      if (position) {
        onCursorPositionChange({
          lineNumber: position.lineNumber,
          column: position.column,
        });
      }

      // Subscribe to cursor changes
      editor.onDidChangeCursorPosition((e) => {
        onCursorPositionChange({
          lineNumber: e.position.lineNumber,
          column: e.position.column,
        });
      });
    }
  };

  // Detect dark mode
  const isDarkMode =
    typeof window !== 'undefined' &&
    document.documentElement.classList.contains('dark');

  return (
    <div className={cn('h-full', className)}>
      <MonacoEditor
        height="100%"
        language="yaml"
        theme={isDarkMode ? 'vs-dark' : 'vs'}
        value={value}
        onChange={readOnly ? undefined : onChange}
        onMount={editorDidMount}
        options={{
          readOnly: readOnly,
          // automaticLayout: true,
          minimap: { enabled: false },
          scrollBeyondLastLine: false,
          quickSuggestions: readOnly
            ? false
            : { other: true, comments: false, strings: true },
          formatOnType: !readOnly,
          formatOnPaste: !readOnly,
          renderValidationDecorations: readOnly ? 'off' : 'on',
          lineNumbers: lineNumbers ? 'on' : 'off',
          glyphMargin: true,
          fontFamily:
            "'JetBrains Mono', 'Fira Code', Menlo, Monaco, 'Courier New', monospace",
          fontSize: 13,
          padding: {
            top: 8,
            bottom: 8,
          },
        }}
      />
    </div>
  );
}

export default DAGEditor;
