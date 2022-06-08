import { Button, Stack, TextField, Typography } from "@mui/material";
import React from "react";
import { Modal } from "@mui/material";
import Box from "@mui/material/Box";
import { View } from "../models/View";

type Props = {
  refresh: () => void;
};

type EditingView = {
  name: string;
  desc: string;
  tags: string;
};

function CreateViewButton({ refresh }: Props) {
  const [modalOpen, setModalOpen] = React.useState(false);
  const [editingView, setEditingView] = React.useState<Partial<EditingView>>(
    {}
  );
  const onCreateView = React.useCallback(async () => {
    const { name, tags, desc } = editingView;
    if (!name || name.trim() === "") {
      alert("View name must be input");
      return;
    }
    if (!tags || tags.trim() === "") {
      alert("Tags must be input");
      return;
    }
    const view: View = {
      Name: name,
      Desc: desc || "",
      ContainTags: tags.split(",").map((t) => t.trim().toLowerCase()),
    };
    const url = `${API_URL}/views`;
    const resp = await fetch(url, {
      method: "PUT",
      mode: "cors",
      headers: {
        Accept: "application/json",
      },
      body: JSON.stringify(view),
    });
    setModalOpen(false);
    if (resp.ok) {
      refresh();
    } else {
      const e = await resp.text();
      alert(e);
    }
  }, [editingView]);
  return (
    <React.Fragment>
      <Button
        variant="contained"
        size="small"
        sx={{
          width: "100px",
          border: 0,
        }}
        onClick={async () => {
          setEditingView({});
          setModalOpen(true);
        }}
      >
        New
      </Button>
      <Modal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        aria-labelledby="modal-modal-title"
        aria-describedby="modal-modal-description"
      >
        <Box sx={style}>
          <Typography id="modal-modal-title" variant="h6" component="h2">
            New View
          </Typography>
          <Stack
            direction={"column"}
            spacing={2}
            sx={{
              flexDirection: "column",
              justifyContent: "flex-start",
              alignItems: "center",
              mt: 2,
            }}
          >
            <TextField
              required
              label="View Name"
              placeholder="View Name"
              defaultValue=""
              sx={{
                width: "100%",
              }}
              onChange={(e) => {
                setEditingView({
                  ...editingView,
                  name: e.target.value,
                });
              }}
            />
            <TextField
              label="Description"
              placeholder=""
              defaultValue=""
              sx={{
                width: "100%",
              }}
              onChange={(e) => {
                setEditingView({
                  ...editingView,
                  desc: e.target.value,
                });
              }}
            />
            <TextField
              required
              label="Tag(s)"
              defaultValue=""
              placeholder="tag1,tag2"
              sx={{
                width: "100%",
              }}
              onChange={(e) => {
                setEditingView({
                  ...editingView,
                  tags: e.target.value,
                });
              }}
            />
            <Button onClick={onCreateView}>Create</Button>
          </Stack>
        </Box>
      </Modal>
    </React.Fragment>
  );
}

export default CreateViewButton;

const style = {
  position: "absolute" as "absolute",
  top: "50%",
  left: "50%",
  transform: "translate(-50%, -50%)",
  width: 400,
  bgcolor: "background.paper",
  border: "2px solid #000",
  boxShadow: 24,
  p: 4,
};
