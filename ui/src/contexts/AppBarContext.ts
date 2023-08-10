import React from 'react';

type AppBarContextType = {
  title: string;
  setTitle(val: string): void;
};

export const AppBarContext = React.createContext<AppBarContextType>({
  title: '',
  setTitle: () => {
    return;
  },
});
