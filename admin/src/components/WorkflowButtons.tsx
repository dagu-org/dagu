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
  const buttonStyle = React.useMemo(
    () => ({
      start: {
        width: "100px",
        backgroundColor: "gray",
        border: 0,
        color: "white",
      },
      stop: {
        width: "100px",
        backgroundColor: "gray",
        border: 0,
        color: "white",
      },
      retry: {
        width: "100px",
        backgroundColor: "gray",
        border: 0,
        color: "white",
      },
    }),
    []
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
    <div className="mr-4 pt-4 is-flex is-flex-direction-row">
      <button
        value="start"
        className="button is-rounded"
        disabled={!buttonState["start"]}
        style={buttonStyle["start"]}
        onClick={() =>
          onSubmit("Do you really want to start the workflow?", {
            group: group,
            name: name,
            action: "start",
          })
        }
      >
        <span className="icon">
          <i className="fa-solid fa-play"></i>
        </span>
        <span>Start</span>
      </button>
      <input type="hidden" name="group" value={group}></input>
      <button
        value="stop"
        className="button is-rounded ml-4"
        disabled={!buttonState["stop"]}
        style={buttonStyle["stop"]}
        onClick={() =>
          onSubmit("Do you really want to cancel the workflow?", {
            group: group,
            name: name,
            action: "stop",
          })
        }
      >
        <span className="icon">
          <i className="fa-solid fa-stop"></i>
        </span>
        <span>Stop</span>
      </button>
      <button
        value="retry"
        className="button is-rounded ml-4"
        disabled={!buttonState["retry"]}
        style={buttonStyle["retry"]}
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
        <span className="icon">
          <i className="fa-solid fa-reply"></i>
        </span>
        <span>Retry</span>
      </button>
    </div>
  );
}
export default WorkflowButtons;
