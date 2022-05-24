import React from "react";
import Prism from "../assets/js/prism"

type Props = {
  value: string;
  onEdit: () => void;
};

function ConfigPreview({ value, onEdit }: Props) {
  React.useEffect(() => {
    Prism.highlightAll();
  }, [value]);
  return (
    <div>
      <pre>
        <code className="language-yaml">{value}</code>
      </pre>
      <button className="button is-info" onClick={onEdit}>
        Edit
      </button>
    </div>
  );
}

export default ConfigPreview;
