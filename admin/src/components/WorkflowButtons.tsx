import { Box, Button, Stack } from "@mui/material";
import React, { FormEvent } from "react";
import { GetWorkflowResponse } from "../api/Workflow";
import { WorkflowContext } from "../contexts/WorkflowContext";
import { SchedulerStatus, Status } from "../models/Status";

type Props = {
  status?: Status;
  group: string;
  name: string;
  refresh: () => void;
};

function WorkflowButtons({ status, group, name, refresh }: Props) {
  const onSubmit = React.useCallback(
    async (
      warn: string,
      params: {
        group: string;
        name: string;
        action: string;
        requestId?: string;
      }
    ) => {
      if (!confirm(warn)) {
        return;
      }
      const form = new FormData();
      form.set("group", params.group);
      form.set("action", params.action);
      if (params.requestId) {
        form.set("request-id", params.requestId);
      }
      const url = `${API_URL}/dags/${params.name}`;
      const ret = await fetch(url, {
        method: "POST",
        mode: "cors",
        body: form,
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
  const buttonState = React.useMemo(
    () => ({
      start: status?.Status != SchedulerStatus.Running,
      stop: status?.Status == SchedulerStatus.Running,
      retry:
        status?.Status != SchedulerStatus.Running && status?.RequestId != "",
    }),
    [status]
  );
  return (
    <Stack direction="row" spacing={2}>
      <Button
        variant="contained"
        color="info"
        size="small"
        startIcon={
          <span className="icon">
            <i className="fa-solid fa-play"></i>
          </span>
        }
        disabled={!buttonState["start"]}
        onClick={() =>
          onSubmit("Do you really want to start the workflow?", {
            group: group,
            name: name,
            action: "start",
          })
        }
      >
        Start
      </Button>
      <Button
        variant="contained"
        color="info"
        size="small"
        startIcon={
          <span className="icon">
            <i className="fa-solid fa-stop"></i>
          </span>
        }
        disabled={!buttonState["stop"]}
        onClick={() =>
          onSubmit("Do you really want to cancel the workflow?", {
            group: group,
            name: name,
            action: "stop",
          })
        }
      >
        Stop
      </Button>
      <Button
        variant="contained"
        color="info"
        size="small"
        startIcon={
          <span className="icon">
            <i className="fa-solid fa-reply"></i>
          </span>
        }
        disabled={!buttonState["retry"]}
        onClick={() =>
          onSubmit(
            `Do you really want to rerun the last execution (${status?.RequestId}) ?`,
            {
              group: group,
              name: name,
              requestId: status?.RequestId,
              action: "retry",
            }
          )
        }
      >
        Retry
      </Button>
    </Stack>
  );
}
export default WorkflowButtons;
