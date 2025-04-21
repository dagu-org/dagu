import { Box, Button, Modal, Stack, Typography } from '@mui/material';
import React from 'react';
import { components, NodeStatus } from '../../api/v2/schema';

type Props = {
  visible: boolean;
  dismissModal: () => void;
  step?: components['schemas']['Step'];
  onSubmit: (step: components['schemas']['Step'], status: NodeStatus) => void;
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

function StatusUpdateModal({ visible, dismissModal, step, onSubmit }: Props) {
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
  if (!step) {
    return null;
  }
  return (
    <Modal open={visible} onClose={dismissModal}>
      <Box sx={style}>
        <Stack direction="row" alignContent="center" justifyContent="center">
          <Typography variant="h6">Update status of "{step.name}"</Typography>
        </Stack>
        <Stack
          direction="column"
          alignContent="center"
          justifyContent="center"
          spacing={2}
          mt={2}
        >
          <Stack
            direction="row"
            alignContent="center"
            justifyContent="center"
            spacing={2}
          >
            <Button
              variant="outlined"
              onClick={() => onSubmit(step, NodeStatus.Success)}
            >
              Mark Success
            </Button>
            <Button
              variant="outlined"
              onClick={() => onSubmit(step, NodeStatus.Failed)}
            >
              Mark Failed
            </Button>
          </Stack>
          <Stack direction="row" alignContent="center" justifyContent="center">
            <Button variant="outlined" color="error" onClick={dismissModal}>
              Cancel
            </Button>
          </Stack>
        </Stack>
      </Box>
    </Modal>
  );
}

export default StatusUpdateModal;
