import React from 'react';

type Props = {
  errors: string[];
};

function DAGSpecErrors({ errors }: Props) {
  if (!errors || errors.length == 0) {
    return null;
  }
  return (
    <div className="notification is-danger mt-0 mb-0">
      <div>Please check the below errors!</div>
      <div className="content">
        <ul>
          {errors.map((e, i) => (
            <li key={`${i}`}>{e}</li>
          ))}
        </ul>
      </div>
    </div>
  );
}

export default DAGSpecErrors;
