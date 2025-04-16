/**
 * HistoryTableRow component renders a single row in the execution history table.
 *
 * @module features/dags/components/dag-execution
 */
import { TableCell } from '@mui/material';
import React, { CSSProperties } from 'react';
import { statusColorMapping } from '../../../../consts';
import StyledTableRow from '../../../../ui/StyledTableRow';
import { components } from '../../../../api/v2/schema';

/**
 * Props for the HistoryTableRow component
 */
type Props = {
  /** Grid data for the row */
  data: components['schemas']['DAGGridItem'];
  /** Callback for when a cell is selected */
  onSelect: (idx: number) => void;
  /** Currently selected index */
  idx: number;
};

/**
 * Style for the status circles
 */
const circleStyle = {
  width: '14px',
  height: '14px',
  borderRadius: '50%',
  backgroundColor: '#000000',
};

/**
 * HistoryTableRow displays a row in the execution history table
 * with colored circles representing the status of each run
 */
function HistoryTableRow({ data, onSelect, idx }: Props) {
  return (
    <StyledTableRow>
      <TableCell>{data.name}</TableCell>
      {[...data.history].reverse().map((status, i) => {
        const style: CSSProperties = { ...circleStyle };
        const tdStyle: CSSProperties = { maxWidth: '22px' };

        // Highlight the selected cell
        if (i == idx) {
          tdStyle.backgroundColor = '#FFDDAD';
        }

        // Set the color based on status
        if (status != 0) {
          style.backgroundColor = statusColorMapping[status]?.backgroundColor;
          style.color = statusColorMapping[status]?.color;
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
