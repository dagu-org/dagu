import React from "react";
import MonacoEditor from "react-monaco-editor";

type Props = {
  value: string;
  onChange: (value: string) => void;
};

function ConfigEditor({ value, onChange }: Props) {
  return (
    <MonacoEditor height="60vh" defaultValue={value} onChange={onChange} />
  );
}

export default ConfigEditor;
