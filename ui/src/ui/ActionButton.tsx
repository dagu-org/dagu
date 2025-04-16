import { Button, IconButton } from '@mui/material';
import React, { ReactElement } from 'react';

interface ActionButtonProps {
  children: React.ReactNode;
  label: boolean;
  icon: ReactElement;
  disabled: boolean;
  onClick: () => void;
}

export default function ActionButton({
  label,
  children,
  icon,
  disabled,
  onClick,
}: ActionButtonProps) {
  return label ? (
    <Button
      variant="outlined"
      color="primary"
      size="small"
      startIcon={icon}
      disabled={disabled}
      onClick={onClick}
    >
      {children}
    </Button>
  ) : (
    <IconButton
      color="primary"
      size="small"
      onClick={onClick}
      disabled={disabled}
    >
      {icon}
    </IconButton>
  );
}
