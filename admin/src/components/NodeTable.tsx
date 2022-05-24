import React, { CSSProperties } from "react";
import { stepTabColStyles } from "../consts";
import { Node } from "../models/Node";
import { SchedulerStatus, Status } from "../models/Status";
import { Step } from "../models/Step";
import NodeTableRow from "./NodeTableRow";

type Props = {
  nodes?: Node[];
  file?: string;
  status: Status;
  name: string;
  group: string;
  refresh: () => void;
};

function NodeTable({ nodes, status, group, name, refresh, file = "" }: Props) {
  const [modal, setModal] = React.useState(false);
  const [current, setCurrent] = React.useState<Step | null>(null);
  const requireModal = (step: Step) => {
    if (
      status?.Status != SchedulerStatus.Running &&
      status?.Status != SchedulerStatus.None
    ) {
      setCurrent(step);
      setModal(true);
    }
  };
  const dismissModal = React.useCallback(() => {
    setModal(false);
  }, [setModal]);
  React.useEffect(() => {
    const callback = (event: KeyboardEvent) => {
      const e = event || window.event;
      if (e.key == "Escape" || e.key == "Esc") {
        setModal(false);
      }
    };
    document.addEventListener("keydown", callback);
    return () => {
      document.removeEventListener("keydown", callback);
    };
  }, []);
  const onUpdateStatus = React.useCallback(
    async (params: {
      group: string;
      name: string;
      step: string;
      action: string;
      requestId: string;
    }) => {
      const form = new FormData();
      form.set("group", params.group);
      form.set("action", params.action);
      form.set("request-id", params.requestId);
      form.set("step", params.step);
      const url = `${API_URL}/dags/${params.name}`;
      const ret = await fetch(url, {
        method: "POST",
        mode: "cors",
        body: form,
      });
      if (ret.ok) {
        refresh();
        dismissModal();
      } else {
        const e = await ret.text();
        alert(e);
      }
    },
    [refresh, dismissModal]
  );
  const tableStyle: CSSProperties = {
    tableLayout: "fixed",
    wordWrap: "break-word",
  };
  const divStyle: CSSProperties = {
    overflowX: "auto",
  };
  const styles = stepTabColStyles;
  const modalbuttonStyle = {};
  const modalStyle = {
    display: modal ? "flex" : "none",
  };
  let i = 0;
  if (!nodes || !nodes.length) {
    return null;
  }

  return (
    <div className="card mt-4" style={divStyle}>
      <table className="table is-bordered is-fullwidth card" style={tableStyle}>
        <thead className="has-background-light">
          <tr>
            <th style={styles[i++]}>#</th>
            <th style={styles[i++]}>Step Name</th>
            <th style={styles[i++]}>Description</th>
            <th style={styles[i++]}>Command</th>
            <th style={styles[i++]}>Args</th>
            <th style={styles[i++]}>Started At</th>
            <th style={styles[i++]}>Finished At</th>
            <th style={styles[i++]}>Status</th>
            <th style={styles[i++]}>Error</th>
            <th style={styles[i++]}>Log</th>
          </tr>
        </thead>
        <tbody>
          {nodes.map((n, idx) => (
            <NodeTableRow
              key={n.Step.Name}
              rownum={idx + 1}
              node={n}
              file={file}
              group={group}
              name={name}
              onRequireModal={requireModal}
            ></NodeTableRow>
          ))}
        </tbody>
      </table>

      {current ? (
        <div className="modal is-active" style={modalStyle}>
          <div className="modal-background"></div>
          <div className="modal-card">
            <header className="modal-card-head">
              <p className="modal-card-title">
                Update status of "{current.Name}"
              </p>
              <button
                className="delete"
                aria-label="close"
                onClick={dismissModal}
              ></button>
            </header>
            <section className="modal-card-body">
              <div className="mr-4 pt-4 is-flex is-flex-direction-row">
                <button
                  value="mark-success"
                  className="button is-info"
                  style={modalbuttonStyle}
                  onClick={() =>
                    onUpdateStatus({
                      group,
                      name,
                      requestId: status.RequestId,
                      action: "mark-success",
                      step: current.Name,
                    })
                  }
                >
                  <span>Mark Success</span>
                </button>
                <button
                  className="button is-info ml-4"
                  style={modalbuttonStyle}
                  onClick={() =>
                    onUpdateStatus({
                      group,
                      name,
                      requestId: status.RequestId,
                      action: "mark-failed",
                      step: current.Name,
                    })
                  }
                >
                  <span>Mark Failed</span>
                </button>
              </div>
            </section>
            <footer className="modal-card-foot">
              <button className="button" onClick={dismissModal}>
                Cancel
              </button>
            </footer>
          </div>
        </div>
      ) : null}
    </div>
  );
}

export default NodeTable;
