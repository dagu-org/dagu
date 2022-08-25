import React, { useEffect, useRef } from 'react';
import { Box, Button, Grid, Stack, TextField } from '@mui/material';
import { AppBarContext } from '../../contexts/AppBarContext';
import useSWR from 'swr';
import Title from '../../components/atoms/Title';

function Search() {
  const appBarContext = React.useContext(AppBarContext);
  const [searchVal, setSearchVal] = React.useState('');

  const ref = useRef<HTMLInputElement>(null);

  useEffect(() => {
    ref.current?.focus();
  }, [ref.current]);

  return (
    <Grid container spacing={3} sx={{ mx: 4, width: '100%' }}>
      <Grid item xs={12}>
        <Title>Search</Title>
        <Stack spacing={2} direction="row">
          <TextField
            label="Search Text"
            variant="outlined"
            style={{
              flex: 0.5,
            }}
            inputRef={ref}
            InputProps={{
              value: searchVal,
              onChange: (e) => {
                setSearchVal(e.target.value);
              },
              type: 'search',
            }}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                console.log('submit');
              }
            }}
          />
          <Button
            variant="contained"
            sx={{
              width: '100px',
              border: 0,
            }}
            onClick={async () => {}}
          >
            Search
          </Button>
        </Stack>
      </Grid>
    </Grid>
  );
}
export default Search;
