import { CircularProgress, Container } from "@mui/material";
import React from "react";

type Props = {
  children?: JSX.Element | JSX.Element[];
  loaded: boolean;
};

function WithLoading({ children, loaded }: Props) {
  if (!loaded) {
    return (
      <Container
        sx={{
          width: "100%",
        }}
      >
        <CircularProgress />
      </Container>
    );
  }
  return <React.Fragment>{children}</React.Fragment>;
}

export default WithLoading;
