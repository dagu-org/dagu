import React from "react";
import { ProgressPlugin } from "webpack";
import { WorkflowContext } from "../contexts/WorkflowContext";
import { WorkflowTabType } from "../models/WorkflowTab";
import ConfigEditButtons from "./ConfigEditButtons";

type Props = {
  tab: WorkflowTabType;
  group: string;
};

const VisibleTabs = [
  ["Status", WorkflowTabType.Status],
  ["Config", WorkflowTabType.Config],
  ["History", WorkflowTabType.History],
];

function WorkflowTabs({ tab, group }: Props) {
  const classes = VisibleTabs.map((elem) =>
    elem[1] == tab ? "is-active" : ""
  );
  return (
    <div className="has-background-white is-flex is-flex-direction-row is-justify-content-space-between is-align-items-center pt-2 pb-2">
      <div className="tabs is-toggle mb-0 px-3 has-text-weight-semibold">
        <ul>
          {VisibleTabs.map((elem, i) => {
            const c = classes[i];
            const href = `?group=${encodeURI(group)}&t=${elem[1]}`;
            return (
              <li key={href} className={c}>
                <a href={href}>{elem[0]}</a>
              </li>
            );
          })}
        </ul>
      </div>
      <WorkflowContext.Consumer>
        {(props) => (
          <ConfigEditButtons
            tab={tab}
            group={group}
            name={props.name}
          ></ConfigEditButtons>
        )}
      </WorkflowContext.Consumer>
    </div>
  );
}

export default WorkflowTabs;
