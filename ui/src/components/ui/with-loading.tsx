import React from 'react';
import LoadingIndicator from './loading-indicator';

type Props = {
  children?: React.JSX.Element | React.JSX.Element[] | null;
  loaded: boolean;
};

function WithLoading({ children, loaded }: Props) {
  return loaded ? (
    <React.Fragment>{children}</React.Fragment>
  ) : (
    <LoadingIndicator />
  );
}

export default WithLoading;
