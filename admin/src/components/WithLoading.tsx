import React from 'react';
import Loading from './Loading';

type Props = {
  children?: JSX.Element | JSX.Element[] | null;
  loaded: boolean;
};

function WithLoading({ children, loaded }: Props) {
  return loaded ? <React.Fragment>{children}</React.Fragment> : <Loading />;
}

export default WithLoading;
