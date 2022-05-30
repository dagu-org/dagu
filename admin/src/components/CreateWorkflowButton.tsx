import { Button } from "@mui/material";
import React from "react";

type Props = {
  refresh: () => void;
};

function CreateWorkflowButton({ refresh }: Props) {
  return (
    <Button
      variant="contained"
      size="small"
      sx={{
        width: "100px",
        border: 0,
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
      New
    </Button>
  );
}
export default CreateWorkflowButton;
