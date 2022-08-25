import { Divider, List, ListItem, Stack, Typography } from '@mui/material';
import React, { ReactElement, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { SearchResult } from '../../models/api';
import DAGDefinition from './DAGDefinition';
import Prism from '../../assets/js/prism';

type Props = {
  results: SearchResult[];
};

function SearchResult({ results }: Props) {
  const elements = React.useMemo(
    () =>
      results.map((result) => {
        const keys = Object.keys(result.Matched);
        const ret = [] as ReactElement[];
        keys.forEach((k, i) => {
          const m = result.Matched[k];
          ret.push(
            <ListItem key={`${result.Name}-${k}-${i}`}>
              <Stack direction="column" spacing={1} style={{ width: '100%' }}>
                <Link to={`/dags/${encodeURI(result.Name)}/spec`}>
                  <Typography variant="h6">{result.Name}</Typography>
                </Link>
                <DAGDefinition
                  value={m}
                  lineNumbers
                  startLine={+k}
                  keyword={'step'}
                  noHighlight
                />
              </Stack>
            </ListItem>
          );
        });
        return ret;
      }),
    [results]
  );
  useEffect(() => Prism.highlightAll(), [elements]);
  return <List>{elements}</List>;
}
export default SearchResult;
