import React, { createContext, useContext, useState, useCallback } from 'react';

interface UnsavedChangesContextType {
  hasUnsavedChanges: boolean;
  setHasUnsavedChanges: (value: boolean) => void;
}

const UnsavedChangesContext = createContext<UnsavedChangesContextType | null>(null);

export function useUnsavedChanges() {
  const context = useContext(UnsavedChangesContext);
  if (!context) {
    // Return a default object when used outside provider
    return { hasUnsavedChanges: false, setHasUnsavedChanges: () => {} };
  }
  return context;
}

export function UnsavedChangesProvider({ children }: { children: React.ReactNode }) {
  const [hasUnsavedChanges, setHasUnsavedChangesState] = useState(false);

  const setHasUnsavedChanges = useCallback((value: boolean) => {
    setHasUnsavedChangesState(value);
  }, []);

  return (
    <UnsavedChangesContext.Provider value={{ hasUnsavedChanges, setHasUnsavedChanges }}>
      {children}
    </UnsavedChangesContext.Provider>
  );
}
