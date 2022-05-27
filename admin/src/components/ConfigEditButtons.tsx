import React from "react";
import { WorkflowTabType } from "../models/WorkflowTab";

type Props = {
  tab: WorkflowTabType;
  group: string;
  name: string;
};

function ConfigEditButtons({ tab, group, name }: Props) {
  const buttonStyle = React.useMemo(
    () => ({
      rename: { width: "100px" },
    }),
    []
  );
  if (tab != WorkflowTabType.Config) {
    return null;
  }
  return (
    <div className="mt-0 mb-0 mr-4 is-flex is-flex-direction-row">
      <button
        type="submit"
        name="action"
        value="rename"
        className="button is-info is-small is-outlined"
        disabled={false}
        style={buttonStyle["rename"]}
        onClick={async () => {
          const val = window.prompt(
            "Please input the new file name (*.yaml)",
            ""
          );
          if (!val) {
            return;
          }
          if (val.indexOf(" ") != -1) {
            alert("File name cannot contain space");
            return;
          }
          const formData = new FormData();
          formData.append("action", "rename");
          formData.append("group", group);
          formData.append("value", val);
          const url = `${API_URL}/dags/${name}`;
          const resp = await fetch(url, {
            method: "POST",
            headers: { Accept: "application/json" },
            body: formData,
          });
          if (resp.ok) {
            window.location.href = `/dags/${val.replace(/.yaml$/, "")}`;
          } else {
            const e = await resp.text();
            alert(e);
          }
        }}
      >
        <span>Rename</span>
      </button>
    </div>
  );
}

export default ConfigEditButtons;
