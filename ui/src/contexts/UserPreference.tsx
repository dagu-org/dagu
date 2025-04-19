import React, { createContext, useCallback, useContext, useState } from 'react';

export type UserPreferences = {
  pageLimit: number;
};

const UserPreferencesContext = createContext<{
  preferences: UserPreferences;
  updatePreference: <K extends keyof UserPreferences>(
    key: K,
    value: UserPreferences[K]
  ) => void;
}>(null!);

export function UserPreferencesProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const [preferences, setPreferences] = useState<UserPreferences>(() => {
    try {
      const saved = localStorage.getItem('user_preferences');
      const defaultPrefs = {
        pageLimit: 50,
      };
      return saved ? { ...defaultPrefs, ...JSON.parse(saved) } : defaultPrefs;
    } catch {
      // Fallback to defaults if parsing fails
      return {
        pageLimit: 50,
      };
    }
  });

  const updatePreference = useCallback(
    <K extends keyof UserPreferences>(key: K, value: UserPreferences[K]) => {
      setPreferences((prev) => {
        const next = { ...prev, [key]: value };
        localStorage.setItem('user_preferences', JSON.stringify(next));
        return next;
      });
    },
    []
  );

  return (
    <UserPreferencesContext.Provider value={{ preferences, updatePreference }}>
      {children}
    </UserPreferencesContext.Provider>
  );
}

export function useUserPreferences() {
  return useContext(UserPreferencesContext);
}
