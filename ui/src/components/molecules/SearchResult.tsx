import { Divider, List, ListItem, Stack, Typography } from '@mui/material';
import React, { ReactElement, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { SearchResult } from '../../models/api';
import DAGDefinition from './DAGDefinition';
import Prism from '../../assets/js/prism';
import { components } from '../../api/v2/schema';

type Props = {
  results: components['schemas']['SearchDAGsResultItem'][];
};

function SearchResult({ results }: Props) {
  const elements = React.useMemo(
    () =>
      results.map((result, i) => {
        const ret = [] as ReactElement[];
        result.matches.forEach((m, j) => {
          ret.push(
            <ListItem key={`${result.name}-${m.lineNumber}`}>
              <Stack direction="column" spacing={1} style={{ width: '100%' }}>
                {j == 0 ? (
                  <Link to={`/dags/${encodeURI(result.name)}/spec`}>
                    <Typography variant="h6">{result.name}</Typography>
                  </Link>
                ) : null}
                <DAGDefinition
                  value={m.line}
                  lineNumbers
                  startLine={m.startLine}
                  highlightLine={m.lineNumber - m.startLine}
                  noHighlight
                />
              </Stack>
            </ListItem>
          );
        });
        if (i < results.length - 1) {
          ret.push(<Divider key={`${result.name}-divider`} />);
        }
        return ret;
      }),
    [results]
  );
  useEffect(() => Prism.highlightAll(), [elements]);
  return <List>{elements}</List>;
}
export default SearchResult;
