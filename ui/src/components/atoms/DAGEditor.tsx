import React, { useEffect, useRef } from 'react';
import MonacoEditor from 'react-monaco-editor';
import * as monaco from 'monaco-editor';
import { setDiagnosticsOptions } from 'monaco-yaml';

type Props = {
  value: string;
  onChange: (value: string) => void;
};

function DAGEditor({ value, onChange }: Props) {
  const editorRef = useRef<monaco.editor.IStandaloneCodeEditor | null>(null);

  useEffect(() => {
    setDiagnosticsOptions({
      enableSchemaRequest: true,
      schemas: [
        {
          uri: 'https://raw.githubusercontent.com/daguflow/dagu/main/schemas/dag.schema.json',
          fileMatch: ['*'],
        },
      ],
    });

    // cleanup
    return () => {
      if (editorRef.current) {
        editorRef.current.dispose();
      }
    };
  }, []);

  const editorDidMount = (editor: monaco.editor.IStandaloneCodeEditor) => {
    editorRef.current = editor;
  };

  return (
    <MonacoEditor
      height="60vh"
      language="yaml"
      value={value}
      onChange={onChange}
      editorDidMount={editorDidMount}
      options={{
        automaticLayout: true,
        scrollBeyondLastLine: false,
        quickSuggestions: { other: true, comments: false, strings: true },
        formatOnType: true,
      }}
    />
  );
}

export default DAGEditor;
