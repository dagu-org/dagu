import React from 'react';
import Prism from '../assets/js/prism';

type Props = {
  value: string;
};

function ConfigPreview({ value }: Props) {
  React.useEffect(() => {
    Prism.highlightAll();
  }, [value]);
  return (
    <pre
      style={{
        fontSize: '0.9rem',
      }}
    >
      <code className="language-yaml">{value}</code>
    </pre>
  );
}

export default ConfigPreview;
