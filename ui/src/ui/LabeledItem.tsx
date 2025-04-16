import { Stack, Typography } from '@mui/material';
import React, { ReactNode } from 'react';

type LabeldItemProps = {
  label: string;
  children: string | ReactNode;
};

export default function LabeledItem({ label, children }: LabeldItemProps) {
  return (
    <Stack
      direction="row"
      sx={{
        alignItems: 'center',
        justifyContent: 'flex-start',
      }}
    >
      <Typography
        sx={{
          fontWeight: 800,
        }}
      >
        {label}:&nbsp;
      </Typography>
      {typeof children === 'string' ? (
        <Typography>{children}</Typography>
      ) : (
        children
      )}
    </Stack>
  );
}
