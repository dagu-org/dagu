import React from 'react';
import { WorkflowListItem } from '../../models/api';

type Props = {
  DAGs: WorkflowListItem[];
  errors: string[];
  hasError: boolean;
};

function DAGErrors({ DAGs, errors, hasError }: Props) {
  if (!DAGs || !hasError) {
    return <div></div>;
  }
  return (
    <div className="notification is-danger mt-0 mb-0">
      <div>Please check the below errors!</div>
      <div className="content">
        <ul>
          {DAGs.filter((w) => w.ErrorT).map((w) => {
            const url = encodeURI(w.File);
            return (
              <li>
                <a href={url}>{w.File}</a>: {w.ErrorT}{' '}
              </li>
            );
          })}
          {errors.map((e) => (
            <li>{e}</li>
          ))}
        </ul>
      </div>
    </div>
  );
}
export default DAGErrors;
