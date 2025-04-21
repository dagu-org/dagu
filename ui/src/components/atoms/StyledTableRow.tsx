import { TableRow } from '@mui/material';
import { styled } from '@mui/system';

const StyledTableRow = styled(TableRow)(() => ({
  // hide last border
  '&:last-child td, &:last-child th': {
    border: 0,
  },
}));

export default StyledTableRow;
