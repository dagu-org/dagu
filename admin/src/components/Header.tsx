import React from "react";

type Props = {
  refresh: () => void;
};

function Header({ refresh }: Props) {
  return (
    <div>
      <div className="has-background-white py-3">
        <div className="is-flex is-flex-direction-row is-justify-content-space-between">
          <h2 className="title ml-2">Workflows</h2>
          <button
            className="button mr-5"
            style={{
              width: "100px",
              backgroundColor: "chocolate",
              border: 0,
              color: "white",
            }}
            onClick={async () => {
              const name = window.prompt(
                "Please input the new file name (*.yaml)",
                ""
              );
              if (name == "") {
                return;
              }
              if (name?.indexOf(" ") != -1) {
                alert("File name cannot contain space");
                return;
              }
              const formData = new FormData();
              formData.append("action", "new");
              formData.append("value", name);
              const resp = await fetch(API_URL, {
                method: "POST",
                mode: "cors",
                headers: {
                  Accept: "application/json",
                },
                body: formData,
              });
              if (resp.ok) {
                refresh();
              } else {
                const e = await resp.text();
                alert(e);
              }
            }}
          >
            New DAG
          </button>
        </div>
      </div>
    </div>
  );
}
export default Header;
