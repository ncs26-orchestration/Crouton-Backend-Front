import { useEffect, useState } from "react";
import { ReactFlowProvider } from "@xyflow/react";
import { useQueryClient } from "@tanstack/react-query";

import { HelpOverlay } from "./components/HelpOverlay";
import { ShellRail, type ShellSection } from "./components/ShellRail";
import { ToastProvider } from "./components/Toasts";
import { ThemeProvider } from "./lib/theme";
import { api } from "./lib/api";
import { LoginView } from "./views/LoginView";
import { RegisterView } from "./views/RegisterView";
import { OrgSetupView } from "./views/OrgSetupView";
import { OrgView } from "./views/OrgView";
import { SettingsView } from "./views/SettingsView";
import { HomeView } from "./views/HomeView";
import { MyWorkView } from "./views/MyWorkView";
import { RequestsView } from "./views/RequestsView";
import { WorkflowView } from "./views/WorkflowView";
import { AgentsView } from "./views/AgentsView";
import { ReportsView } from "./views/ReportsView";
import { PoliciesView } from "./views/PoliciesView";
import { IntegrationsView } from "./views/IntegrationsView";
import { AuthProvider, useAuth } from "./contexts/AuthContext";
import { OrgProvider } from "./contexts/OrgContext";

// App shell — keeps the operator's persistent surfaces always reachable
// from the left icon rail.
//
//   Home      — projects grid or the current chat
//   Inbox     — tasks assigned to you
//   Workflows — workflow builder (alias for home for now)
//   Agents    — AI agents configured for the org
//   Settings  — deploy-target connectors + theme
//   Help      — keyboard-shortcut overlay

const STORAGE_KEY = "aios.lastLocation";

interface Location {
  section: ShellSection;
  requestId: string | null;
  nodeId: string | null;
}

function loadLocation(): Location {
  if (typeof window === "undefined") {
    return { section: "home", requestId: null, nodeId: null };
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return { section: "home", requestId: null, nodeId: null };
    const parsed = JSON.parse(raw);
    const validSections: ShellSection[] = [
      "home", "my-work", "requests", "workflows", "agents",
      "reports", "policies", "integrations", "teams", "settings", "help",
    ];
    return {
      section: validSections.includes(parsed.section) ? parsed.section : "home",
      requestId: typeof parsed.requestId === "string" ? parsed.requestId : null,
      nodeId: typeof parsed.nodeId === "string" ? parsed.nodeId : null,
    };
  } catch {
    return { section: "home", requestId: null, nodeId: null };
  }
}

function saveLocation(loc: Location) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(loc));
}

export function App() {
  return (
    <ThemeProvider>
      <ToastProvider>
        <ReactFlowProvider>
          <AuthProvider>
            <AppRoot />
          </AuthProvider>
        </ReactFlowProvider>
      </ToastProvider>
    </ThemeProvider>
  );
}

// Decides whether to show auth pages, org setup, or the main shell.
//
// Flow:
//   1. No token → login / register
//   2. Token present, orgs loading → full-screen spinner
//   3. Token + no orgs → OrgSetupView (one-time after registration)
//   4. Token + org exists → Shell (wrapped in OrgProvider)
function AppRoot() {
  const { user, logout } = useAuth();
  const [authPage, setAuthPage] = useState<"login" | "register">("login");
  const [orgs, setOrgs] = useState<null | Array<{ id: string; name: string; slug: string; role: string }>>(null);

  useEffect(() => {
    if (!user) { setOrgs(null); return; }
    setOrgs(null);
    api.listOrgs()
      .then((r) => setOrgs(r.orgs))
      .catch((err: unknown) => {
        // 401/403 means the token is invalid — log out instead of looping
        const msg = err instanceof Error ? err.message : "";
        if (msg.includes("401") || msg.includes("403") || msg.includes("Unauthorized")) {
          logout();
        } else {
          // Network error or other — assume no orgs so user can create one
          setOrgs([]);
        }
      });
  }, [user, logout]);

  // Derive state from the orgs array
  const orgState = orgs === null ? "loading" : orgs.length === 0 ? "none" : "has";

  if (!user) {
    if (authPage === "register") {
      return <RegisterView onGoLogin={() => setAuthPage("login")} />;
    }
    return <LoginView onGoRegister={() => setAuthPage("register")} />;
  }

  if (orgState === "loading") {
    return (
      <div className="h-screen w-screen flex items-center justify-center bg-[var(--color-bg)]">
        <div className="flex flex-col items-center gap-3">
          <div className="size-8 rounded-full border-2 border-[var(--color-brand)] border-t-transparent animate-spin" />
          <p className="text-sm text-[var(--color-fg-muted)]">Loading…</p>
        </div>
      </div>
    );
  }

  if (orgState === "none") {
    return (
      <OrgSetupView
        onDone={(org) => setOrgs([org])}
      />
    );
  }

  const firstOrg = orgs![0]!;
  return (
    <OrgProvider initial={firstOrg}>
      <Shell />
    </OrgProvider>
  );
}

function Shell() {
  const { logout, user } = useAuth();
  const qc = useQueryClient();
  const [location, setLocation] = useState<Location>(loadLocation);
  const [helpOpen, setHelpOpen] = useState(false);

  useEffect(() => {
    qc.clear();
  }, [user?.id, qc]);

  useEffect(() => {
    saveLocation(location);
  }, [location]);

  const setSection = (section: ShellSection) => {
    if (section === "help") {
      setHelpOpen(true);
      return;
    }
    setLocation((prev) => ({ ...prev, section }));
  };

  return (
    <div className="h-screen w-screen overflow-hidden flex bg-[var(--color-bg)] text-[var(--color-fg)]">
      <ShellRail
        active={location.section}
        onSelect={setSection}
        onBrandClick={() => setLocation({ section: "home", requestId: null, nodeId: null })}
        onUserClick={() => { qc.clear(); logout(); }}
      />

      {location.section === "home" && <HomeView />}
      {location.section === "my-work" && <MyWorkView />}
      {location.section === "requests" && <RequestsView />}
      {location.section === "workflows" && <WorkflowView />}
      {location.section === "agents" && <OrgView />}
      {location.section === "reports" && <ReportsView />}
      {location.section === "policies" && <PoliciesView />}
      {location.section === "integrations" && <IntegrationsView />}
      {location.section === "teams" && <OrgView />}
      {location.section === "settings" && <SettingsView scopedProjectId={null} />}

      <HelpOverlay open={helpOpen} onClose={() => setHelpOpen(false)} />
    </div>
  );
}


