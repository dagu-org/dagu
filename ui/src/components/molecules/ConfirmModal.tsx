import { Box, Button, Modal, Stack, Typography } from '@mui/material';
import React from 'react';

type Props = {
  title: string;
  buttonText: string;
  children: React.ReactNode;
  visible: boolean;
  dismissModal: () => void;
  onSubmit: () => void;
};

const style = {
  position: 'absolute',
  top: '50%',
  left: '50%',
  transform: 'translate(-50%, -50%)',
  width: 400,
  bgcolor: 'background.paper',
  border: '2px solid #000',
  boxShadow: 24,
  p: 4,
};

function ConfirmModal({
  children,
  title,
  buttonText,
  visible,
  dismissModal,
  onSubmit,
}: Props) {
  React.useEffect(() => {
    const callback = (event: KeyboardEvent) => {
      const e = event || window.event;
      if (e.key == 'Escape' || e.key == 'Esc') {
        dismissModal();
      }
    };
    document.addEventListener('keydown', callback);
    return () => {
      document.removeEventListener('keydown', callback);
    };
  }, [dismissModal]);

  return (
    <Modal open={visible} onClose={dismissModal}>
      <Box sx={style}>
        <Stack direction="row" alignContent="center" justifyContent="center">
          <Typography variant="h6">{title}</Typography>
        </Stack>
        <Stack
          direction="column"
          alignContent="center"
          justifyContent="center"
          spacing={2}
          mt={2}
        >
          <Box>{children}</Box>
          <Button variant="outlined" onClick={() => onSubmit()}>
            {buttonText}
          </Button>
          <Button variant="outlined" color="error" onClick={dismissModal}>
            Cancel
          </Button>
        </Stack>
      </Box>
    </Modal>
  );
}

export default ConfirmModal;
