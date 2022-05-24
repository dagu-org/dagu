import React from "react";
import MonacoEditor from "react-monaco-editor";

type Props = {
  value: string;
  onChange: (value: string) => void;
  onSave: () => void;
  onCancel: () => void;
};

function ConfigEditor({ value, onChange, onSave, onCancel }: Props) {
  return (
    <div>
      <MonacoEditor height="60vh" defaultValue={value} onChange={onChange} />
      <div className="mt-4">
        <button className="button is-info" onClick={onSave}>
          Save
        </button>
        <button className="button ml-2" onClick={onCancel}>
          Cancel
        </button>
      </div>
    </div>
  );
}

export default ConfigEditor;
