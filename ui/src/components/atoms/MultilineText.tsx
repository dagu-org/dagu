import React from 'react';

type Props = {
  children: string;
};

function MultilineText({ children }: Props) {
  return (
    <React.Fragment>
      {children.split('\n').map((l, i) => (
        <span key={i}>
          {l}
          <br></br>
        </span>
      ))}
    </React.Fragment>
  );
}
export default MultilineText;
