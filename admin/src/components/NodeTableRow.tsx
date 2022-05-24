import React from "react";
import { Node } from "../models/Node";
import { Step } from "../models/Step";
import { WorkflowTabType } from "../models/WorkflowTab";
import MultilineText from "./MultilineText";
import NodeStatusTag from "./NodeStatusTag";

type Props = {
  rownum: number;
  node: Node;
  file: string;
  name: string;
  group: string;
  onRequireModal: (step: Step) => void;
};

function NodeTableRow({
  group,
  name,
  rownum,
  node,
  file,
  onRequireModal,
}: Props) {
  const url = `/dags/${name}?t=${WorkflowTabType.StepLog}&group=${group}&file=${file}&step=${node.Step.Name}`;
  const buttonStyle = {
    margin: "0px",
    padding: "0px",
    border: "0px",
    background: "none",
    outline: "none",
  };
  return (
    <tr>
      <td> {rownum} </td>
      <td> {node.Step.Name} </td>
      <td>
        <MultilineText>{node.Step.Description}</MultilineText>
      </td>
      <td> {node.Step.Command} </td>
      <td> {node.Step.Args ? node.Step.Args.join(" ") : ""} </td>
      <td> {node.StartedAt} </td>
      <td> {node.FinishedAt} </td>
      <td>
        <button style={buttonStyle} onClick={() => onRequireModal(node.Step)}>
          <NodeStatusTag status={node.Status}>{node.StatusText}</NodeStatusTag>
        </button>
      </td>
      <td> {node.Error} </td>
      <td>
        <a href={url}> {node.Log} </a>
      </td>
    </tr>
  );
}
export default NodeTableRow;
