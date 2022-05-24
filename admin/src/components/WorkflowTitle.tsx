import React from "react";
import { WorkflowContext } from "../contexts/WorkflowContext";
import WorkflowButtons from "./WorkflowButtons";

type Props = {
  title: string;
};

function WorkflowTitle({ title }: Props) {
  return (
    <div className="has-background-white py-3">
      <div className="is-flex is-flex-direction-row is-justify-content-space-between is-align-items-center is-align-content-center">
        <h2 className="title ml-2">{title}</h2>
        <WorkflowContext.Consumer>
          {(props) =>
            props.data?.DAG ? (
              <WorkflowButtons
                status={props.data.DAG.Status}
                group={props.group}
                name={props.name}
                refresh={props.refresh}
              ></WorkflowButtons>
            ) : null
          }
        </WorkflowContext.Consumer>
      </div>
    </div>
  );
}

export default WorkflowTitle;
