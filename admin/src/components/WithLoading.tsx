import React from "react";

type Props = {
  children?: JSX.Element | JSX.Element[];
  loaded: boolean;
};

function WithLoading({ children, loaded }: Props) {
  if (!loaded) {
    return <div>Loading...</div>;
  }
  return <React.Fragment>{children}</React.Fragment>;
}

export default WithLoading;
