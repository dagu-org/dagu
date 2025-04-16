/**
 * DAGEditor component provides a Monaco editor for editing DAG YAML definitions.
 *
 * @module features/dags/components/dag-editor
 */
import React, { useEffect, useRef } from 'react';
import MonacoEditor, { loader } from '@monaco-editor/react';
import * as monaco from 'monaco-editor';
import { configureMonacoYaml } from 'monaco-yaml';

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
  onChange: (value?: string) => void;
};

/**
 * DAGEditor component provides a Monaco editor with YAML schema validation
 * for editing DAG definitions
 */
function DAGEditor({ value, onChange }: Props) {
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
    <MonacoEditor
      height="60vh"
      language="yaml"
      value={value}
      onChange={onChange}
      onMount={editorDidMount}
      options={{
        automaticLayout: true,
        minimap: { enabled: false },
        scrollBeyondLastLine: false,
        quickSuggestions: { other: true, comments: false, strings: true },
        formatOnType: true,
        formatOnPaste: true,
        renderValidationDecorations: 'on',
        lineNumbers: 'on',
        glyphMargin: true,
      }}
    />
  );
}

export default DAGEditor;
