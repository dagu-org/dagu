import React from "react";
import { GetWorkflowResponse } from "../api/Workflow";
import { WorkflowContext } from "../contexts/WorkflowContext";
import { Config } from "../models/Config";
import { DAG } from "../models/Dag";
import { Step } from "../models/Step";
import ConfigEditor from "./ConfigEditor";
import ConfigInfo from "./ConfigInfo";
import ConfigPreview from "./ConfigPreview";
import GraphDag from "./GraphDag";
import StepConfigTable from "./StepConfigTable";

type Props = {
  data: GetWorkflowResponse;
};

function WorkflowTabConfig({ data }: Props) {
  const mermaidStyle = {
    display: "flex",
    alignItems: "flex-center",
    justifyContent: "flex-start",
    width: data?.DAG?.Config
      ? data.DAG.Config.Steps.length * 240 + "px"
      : "100%",
    minWidth: "100%",
    minHeight: "100px",
  };
  const [editing, setEditing] = React.useState(false);
  const [currentValue, setCurrentValue] = React.useState(data.Definition);
  const handlers = getHandlersFromConfig(data.DAG?.Config);
  if (data.DAG?.Config == null) {
    return null;
  }
  return (
    <WorkflowContext.Consumer>
      {(props) =>
        data.DAG &&
        data.DAG.Config && (
          <div>
            <GraphDag steps={data.DAG.Config.Steps} type="config"></GraphDag>
            <ConfigInfo config={data.DAG.Config!}></ConfigInfo>
            <StepConfigTable steps={data.DAG.Config.Steps}></StepConfigTable>
            <StepConfigTable steps={handlers}></StepConfigTable>

            <div className="content">
              <div className="box">
                <h2>{data.DAG.Config.ConfigPath}</h2>
                {editing ? (
                  <ConfigEditor
                    value={data.Definition}
                    onCancel={() => setEditing(false)}
                    onSave={async () => {
                      const formData = new FormData();
                      formData.append("action", "save");
                      formData.append("value", currentValue);
                      const url = `${API_URL}/dags/${props.name}`;
                      const resp = await fetch(url, {
                        method: "POST",
                        headers: {
                          Accept: "application/json",
                        },
                        body: formData,
                      });
                      if (resp.ok) {
                        setEditing(false);
                        props.refresh();
                      } else {
                        const e = await resp.text();
                        alert(e);
                      }
                    }}
                    onChange={(newValue) => {
                      setCurrentValue(newValue);
                    }}
                  ></ConfigEditor>
                ) : (
                  <ConfigPreview
                    value={data.Definition}
                    onEdit={() => {
                      setEditing(true);
                    }}
                  ></ConfigPreview>
                )}
              </div>
            </div>
          </div>
        )
      }
    </WorkflowContext.Consumer>
  );
}
export default WorkflowTabConfig;

function getHandlersFromConfig(cfg?: Config) {
  const r: Step[] = [];
  if (!cfg) {
    return r;
  }
  const h = cfg.HandlerOn;
  if (h.Success) {
    r.push(h.Success);
  }
  if (h.Failure) {
    r.push(h.Failure);
  }
  if (h.Cancel) {
    r.push(h.Cancel);
  }
  if (h.Exit) {
    r.push(h.Exit);
  }
  return r;
}
