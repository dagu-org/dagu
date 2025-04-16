import * as React from 'react';
import Typography from '@mui/material/Typography';

interface TitleProps {
  children?: React.ReactNode;
}

export default function Title(props: TitleProps) {
  return (
    <Typography
      component="h2"
      variant="h4"
      gutterBottom
      sx={{
        fontWeight: '800',
        color: '#404040',
      }}
    >
      {props.children}
    </Typography>
  );
}
