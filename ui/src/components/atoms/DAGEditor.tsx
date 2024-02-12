import React from 'react';
import MonacoEditor from 'react-monaco-editor';

type Props = {
  value: string;
  onChange: (value: string) => void;
};

function DAGEditor({ value, onChange }: Props) {
  return (
    <MonacoEditor
      height="120vh"
      value={value}
      onChange={onChange}
      language="yaml"
    />
  );
}

export default DAGEditor;
