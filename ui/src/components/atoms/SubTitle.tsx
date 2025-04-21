import * as React from 'react';
import Typography from '@mui/material/Typography';

interface SubTitleProps {
  children?: React.ReactNode;
}

export default function SubTitle(props: SubTitleProps) {
  return (
    <Typography
      component="h3"
      variant="h6"
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
