import { Box, BoxProps } from '@mui/material';
import React from 'react';

interface BorderedBoxProps extends BoxProps {
  children?: React.ReactNode;
}
export default function BorderedBox({
  children,
  sx,
  ...props
}: BorderedBoxProps) {
  return (
    <Box
      sx={{
        border: 1,
        borderColor: 'grey.300',
        borderRadius: '6px',
        ...sx,
      }}
      {...props}
    >
      {children}
    </Box>
  );
}
