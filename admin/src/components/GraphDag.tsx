import React from "react";
import { Node, NodeStatus } from "../models/Node";
import { Step } from "../models/Step";
import Mermaid from "./Mermaid";

type Props = {
  type: "status" | "config";
  steps?: Step[] | Node[];
  onClickNode?: (id: string) => void;
};

function GraphDag({ steps, type = "status", onClickNode }: Props) {
  const mermaidStyle = {
    display: "flex",
    alignItems: "flex-center",
    justifyContent: "flex-start",
    width: steps ? steps.length * 240 + "px" : "100%",
    minWidth: "100%",
    minHeight: "100px",
  };
  const graph = React.useMemo(() => {
    if (!steps) {
      return "";
    }
    let dat = ["flowchart LR;"];
    // @ts-ignore
    window.onClickMermaidNode = onClickNode;
    const addNodeFn = (step: Step, status: NodeStatus) => {
      const id = step.Name.replace(/\s/g, "_");
      let c = graphStatusMap[status] || "";
      dat.push(`${id}(${step.Name})${c};`);
      if (step.Depends) {
        step.Depends.forEach((d) => {
          const depId = d.replace(/\s/g, "_");
          dat.push(`${depId}-->${id};`);
        });
      }
      if (onClickNode) {
        dat.push(`click ${id} onClickMermaidNode`);
      }
    };
    if (type == "status") {
      (steps as Node[]).forEach((s) => addNodeFn(s.Step, s.Status));
    } else {
      (steps as Step[]).forEach((s) => addNodeFn(s, NodeStatus.None));
    }
    dat.push("classDef none fill:white,stroke:lightblue,stroke-width:2px");
    dat.push("classDef running fill:white,stroke:lime,stroke-width:2px");
    dat.push("classDef error fill:white,stroke:red,stroke-width:2px");
    dat.push("classDef cancel fill:white,stroke:pink,stroke-width:2px");
    dat.push("classDef done fill:white,stroke:green,stroke-width:2px");
    dat.push("classDef skipped fill:white,stroke:gray,stroke-width:2px");
    return dat.join("\n");
  }, [steps, onClickNode]);
  return <Mermaid style={mermaidStyle}>{graph}</Mermaid>;
}

export default GraphDag;

const graphStatusMap = {
  [NodeStatus.None]: ":::none",
  [NodeStatus.Running]: ":::running",
  [NodeStatus.Error]: ":::error",
  [NodeStatus.Cancel]: ":::cancel",
  [NodeStatus.Success]: ":::done",
  [NodeStatus.Skipped]: ":::skipped",
};
