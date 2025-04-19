import React, { ReactElement, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { DAGDefinition } from '../../dags/components/dag-editor';
import Prism from '../../../assets/js/prism';
import { components } from '../../../api/v2/schema';

type Props = {
  results: components['schemas']['SearchResultItem'][];
};

function SearchResult({ results }: Props) {
  const elements = React.useMemo(
    () =>
      results.map((result, i) => {
        const ret = [] as ReactElement[];
        result.matches.forEach((m, j) => {
          ret.push(
            <li key={`${result.name}-${m.lineNumber}`} className="py-3 px-4">
              <div className="flex flex-col space-y-2 w-full">
                {j == 0 ? (
                  <Link to={`/dags/${encodeURI(result.name)}/spec`}>
                    <h3 className="text-lg font-semibold text-primary">
                      {result.name}
                    </h3>
                  </Link>
                ) : null}
                <DAGDefinition
                  value={m.line}
                  lineNumbers
                  startLine={m.startLine}
                  highlightLine={m.lineNumber - m.startLine}
                  noHighlight
                />
              </div>
            </li>
          );
        });
        if (i < results.length - 1) {
          ret.push(
            <div
              key={`${result.name}-divider`}
              className="h-px bg-gray-200 my-2"
            />
          );
        }
        return ret;
      }),
    [results]
  );

  useEffect(() => Prism.highlightAll(), [elements]);

  return (
    <ul className="divide-y divide-gray-100 rounded-md border">{elements}</ul>
  );
}

export default SearchResult;
