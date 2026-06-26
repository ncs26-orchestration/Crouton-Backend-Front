import { createContext, useContext, useState } from "react";
const OrgContext = createContext(null);
export function OrgProvider({ children, initial }) {
    const [activeOrg, setActiveOrg] = useState(initial);
    return (<OrgContext.Provider value={{ activeOrg, setActiveOrg }}>
      {children}
    </OrgContext.Provider>);
}
// eslint-disable-next-line react-refresh/only-export-components
export function useOrg() {
    const ctx = useContext(OrgContext);
    if (!ctx)
        throw new Error("useOrg must be used within OrgProvider");
    return ctx;
}
