import { TableCell } from '@mui/material';
import React, { CSSProperties } from 'react';
import { GridData } from '../../models/api';
import { statusColorMapping } from '../../consts';
import StyledTableRow from '../atoms/StyledTableRow';

type Props = {
  data: GridData;
  onSelect: (idx: number) => void;
  idx: number;
};

function HistoryTableRow({ data, onSelect, idx }: Props) {
  const vals = React.useMemo(() => {
    return data.Vals.reverse();
  }, [data]);
  return (
    <StyledTableRow>
      <TableCell>{data.Name}</TableCell>
      {vals.map((status, i) => {
        const style: CSSProperties = { ...circleStyle };
        const tdStyle: CSSProperties = { maxWidth: '22px' };
        if (i == idx) {
          tdStyle.backgroundColor = '#FFDDAD';
        }
        if (status != 0) {
          style.backgroundColor = statusColorMapping[status].backgroundColor;
          style.color = statusColorMapping[status].color;
        }
        return (
          <TableCell
            key={i}
            onClick={() => {
              onSelect(i);
            }}
            style={tdStyle}
          >
            {status != 0 ? <div style={style}></div> : ''}
          </TableCell>
        );
      })}
    </StyledTableRow>
  );
}

export default HistoryTableRow;

const circleStyle = {
  width: '14px',
  height: '14px',
  borderRadius: '50%',
  backgroundColor: '#000000',
};
