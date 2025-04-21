/**
 * DAGEditor component provides a Monaco editor for editing DAG YAML definitions.
 *
 * @module features/dags/components/dag-editor
 */
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
  highlightLine,
  className,
}: Props) {
  const editorRef = useRef<monaco.editor.IStandaloneCodeEditor | null>(null);

  // Clean up editor on unmount
  useEffect(() => {
    return () => {
      editorRef.current?.dispose();
    };
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
  };

  return (
    <div
      className={`relative transition-all duration-300 ${
        readOnly
          ? 'border border-transparent bg-slate-50 dark:bg-slate-800/50 rounded-lg'
          : 'border-2 border-blue-400 dark:border-blue-600 bg-white dark:bg-slate-800 rounded-lg shadow-lg shadow-blue-100 dark:shadow-blue-900/20'
      } ${className}`}
    >
      {!readOnly && (
        <div className="absolute top-0 right-0 z-10 bg-blue-500 dark:bg-blue-600 text-white text-xs font-medium px-2 py-1 rounded-bl-md rounded-tr-md">
          EDIT MODE
        </div>
      )}
      <MonacoEditor
        height="400px"
        language="yaml"
        value={value}
        onChange={readOnly ? undefined : onChange}
        onMount={editorDidMount}
        options={{
          readOnly: readOnly,
          automaticLayout: true,
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
            top: readOnly ? 8 : 24,
            bottom: 8,
          },
        }}
        className="rounded-lg overflow-hidden"
      />
    </div>
  );
}

export default DAGEditor;
