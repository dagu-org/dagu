import React from 'react';
import { TableRow } from '@/components/ui/table';
import { cn } from '@/lib/utils';

interface StyledTableRowProps extends React.ComponentProps<typeof TableRow> {
  children?: React.ReactNode;
}

const StyledTableRow = React.forwardRef<
  HTMLTableRowElement,
  StyledTableRowProps
>(({ className, children, ...props }, ref) => {
  return (
    <TableRow ref={ref} className={cn('last:border-0', className)} {...props}>
      {children}
    </TableRow>
  );
});

StyledTableRow.displayName = 'StyledTableRow';

export default StyledTableRow;
