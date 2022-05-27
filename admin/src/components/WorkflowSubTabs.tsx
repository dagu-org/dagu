import React from "react";
import { WorkflowTabType } from "../models/WorkflowTab";

type Props = {
  tab: WorkflowTabType;
  active: number;
  setActive: (tab: number) => void;
};

function WorkflowSubTabs({ tab, active, setActive }: Props) {
  let tabs: string[] = [];
  if (tab == WorkflowTabType.Status) {
    tabs = ["Graph", "Timeline"];
  }
  if (!tabs.length) {
    return null;
  }
  const classes = tabs.map((_, i) => (i == active ? "is-active" : ""));
  return (
    <div className="has-background-white">
      <div className="tabs is-toggle is-small mb-0 px-3 pb-3 has-text-weight-semibold">
        <ul>
          {tabs.map((elem, i) => {
            const c = classes[i];
            return (
              <li key={i} className={c}>
                <a onClick={() => setActive(i)}>{elem}</a>
              </li>
            );
          })}
        </ul>
      </div>
    </div>
  );
}
export default WorkflowSubTabs;
