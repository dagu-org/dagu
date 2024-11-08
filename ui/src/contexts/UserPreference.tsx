import React, { createContext, useCallback, useContext, useState } from 'react';

export type UserPreferences = {
  pageLimit: number;
}

const UserPreferencesContext = createContext<{
  preferences: UserPreferences;
  updatePreference: <K extends keyof UserPreferences>(
    key: K,
    value: UserPreferences[K]
  ) => void;
}>(null!);


export function UserPreferencesProvider({ children }: { children: React.ReactNode }) {
  const [preferences, setPreferences] = useState<UserPreferences>(() => {
    try {
      const saved = localStorage.getItem('user_preferences');
      return saved ? JSON.parse(saved) : { pageLimit: 50, theme: 'light' };
    } catch {
      return { pageLimit: 50, theme: 'light' };
    }
  });

  const updatePreference = useCallback(<K extends keyof UserPreferences>(
    key: K,
    value: UserPreferences[K]
  ) => {
    setPreferences(prev => {
      const next = { ...prev, [key]: value };
      localStorage.setItem('user_preferences', JSON.stringify(next));
      return next;
    });
  }, []);

  return (
    <UserPreferencesContext.Provider value={{ preferences, updatePreference }}>
      {children}
    </UserPreferencesContext.Provider>
  );

}

export function useUserPreferences() {
  return useContext(UserPreferencesContext);
}