import { createContext, useContext, useState, type ReactNode } from "react";

export interface OrgInfo {
  id: string;
  name: string;
  slug: string;
  role: string;
}

interface OrgContextValue {
  activeOrg: OrgInfo | null;
  setActiveOrg: (org: OrgInfo) => void;
}

const OrgContext = createContext<OrgContextValue | null>(null);

export function OrgProvider({ children, initial }: { children: ReactNode; initial: OrgInfo | null }) {
  const [activeOrg, setActiveOrg] = useState<OrgInfo | null>(initial);
  return (
    <OrgContext.Provider value={{ activeOrg, setActiveOrg }}>
      {children}
    </OrgContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export function useOrg(): OrgContextValue {
  const ctx = useContext(OrgContext);
  if (!ctx) throw new Error("useOrg must be used within OrgProvider");
  return ctx;
}
