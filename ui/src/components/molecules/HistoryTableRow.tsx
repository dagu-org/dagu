import { TableCell } from '@mui/material';
import React, { CSSProperties } from 'react';
import { statusColorMapping } from '../../consts';
import StyledTableRow from '../atoms/StyledTableRow';
import { components } from '../../api/v2/schema';

type Props = {
  data: components['schemas']['DAGLogGridItem'];
  onSelect: (idx: number) => void;
  idx: number;
};

function HistoryTableRow({ data, onSelect, idx }: Props) {
  return (
    <StyledTableRow>
      <TableCell>{data.name}</TableCell>
      {[...data.history].reverse().map((status, i) => {
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
