import { Button } from "@mui/material";
import React from "react";

type Props = {
  name: string;
};

function ConfigEditButtons({ name }: Props) {
  return (
    <Button
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
      Rename
    </Button>
  );
}

export default ConfigEditButtons;
