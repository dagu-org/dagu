import { Box, Pagination, TextField } from '@mui/material';
import React from 'react';

type DAGPaginationProps = {
  totalPages: number;
  page: number;
  pageLimit: number;
  pageChange: (page: number) => void;
  onPageLimitChange: (pageLimit: number) => void;
};

const DAGPagination = ({
  totalPages,
  page,
  pageChange,
  pageLimit,
  onPageLimitChange,
}: DAGPaginationProps) => {
  const [inputValue, setInputValue] = React.useState(pageLimit.toString());

  React.useEffect(() => {
    setInputValue(pageLimit.toString());
  }, [pageLimit]);

  const handleChange = (event: React.ChangeEvent<unknown>, value: number) => {
    pageChange(value);
  };

  const handleLimitChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const value = event.target.value;
    setInputValue(value);
  };

  const commitChange = () => {
    const numValue = parseInt(inputValue);
    if (!isNaN(numValue) && numValue > 0) {
      onPageLimitChange(numValue);
    } else {
      setInputValue(pageLimit.toString());
    }
  };

  const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter') {
      commitChange();
      event.preventDefault();
      (event.target as HTMLInputElement).blur(); // Remove focus after Enter
    }
  };

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'row',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 3,
        mt: 2,
      }}
    >
      <TextField
        size="small"
        label="Items per page"
        value={inputValue}
        onChange={handleLimitChange}
        onBlur={commitChange}
        onKeyDown={handleKeyDown}
        inputProps={{
          type: 'number',
          min: '1',
          style: {
            width: '100px',
            textAlign: 'left',
          },
        }}
      />
      <Pagination count={totalPages} page={page} onChange={handleChange} />
    </Box>
  );
};

export default DAGPagination;
