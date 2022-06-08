import { Button, IconButton, Stack } from "@mui/material";
import React, { ReactElement } from "react";
import { SchedulerStatus, Status } from "../models/Status";

type Props = {
  name: string;
  refresh?: () => any;
};

function ViewActions({ name, refresh = () => {} }: Props) {
  const onSubmit = React.useCallback(
    async (warn: string) => {
      if (!confirm(warn)) {
        return;
      }
      const url = `${API_URL}/views/${encodeURI(name)}`;
      const ret = await fetch(url, {
        method: "DELETE",
        mode: "cors",
      });
      if (ret.ok) {
        refresh();
      } else {
        const e = await ret.text();
        alert(e);
      }
    },
    [refresh]
  );
  return (
    <Stack direction="row" spacing={2}>
      <ActionButton
        icon={
          <span className="icon">
            <i className="fa-solid fa-trash"></i>
          </span>
        }
        onClick={() => onSubmit("Do you want to delete the view?")}
      ></ActionButton>
    </Stack>
  );
}
export default ViewActions;

interface ActionButtonProps {
  children?: string;
  label?: boolean;
  icon: ReactElement;
  disabled?: boolean;
  onClick: () => void;
}

function ActionButton({
  icon,
  onClick,
  children,
  disabled = false,
  label = false,
}: ActionButtonProps) {
  return label ? (
    <Button
      variant="contained"
      color="info"
      size="small"
      startIcon={icon}
      disabled={disabled}
      onClick={onClick}
    >
      {children}
    </Button>
  ) : (
    <IconButton color="info" size="small" onClick={onClick} disabled={disabled}>
      {icon}
    </IconButton>
  );
}
