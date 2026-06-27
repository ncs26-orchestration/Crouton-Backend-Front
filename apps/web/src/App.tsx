import { useEffect, useRef, useState } from "react";
import { ReactFlowProvider } from "@xyflow/react";
import { useQueryClient } from "@tanstack/react-query";

import { HelpOverlay } from "./components/HelpOverlay";
import { HowItWorks } from "./components/HowItWorks";
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
import { MachinesView } from "./views/MachinesView";
import { ReportsView } from "./views/ReportsView";
import { PoliciesView } from "./views/PoliciesView";
import { IntegrationsView } from "./views/IntegrationsView";
import { AuthProvider, useAuth } from "./contexts/AuthContext";
import { OrgProvider, useOrg } from "./contexts/OrgContext";

const STORAGE_KEY = "aios.lastLocation";

interface Location {
  section: ShellSection;
  requestId: string | null;
  nodeId: string | null;
}

const VALID_SECTIONS: ShellSection[] = [
  "home", "my-work", "requests", "workflows", "machines", "agents",
  "reports", "policies", "integrations", "teams", "settings", "help",
];

function loadLocation(): Location {
  if (typeof window === "undefined") {
    return { section: "home", requestId: null, nodeId: null };
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return { section: "home", requestId: null, nodeId: null };
    const parsed = JSON.parse(raw);
    return {
      section: VALID_SECTIONS.includes(parsed.section) ? parsed.section : "home",
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
  const { activeOrg } = useOrg();
  const qc = useQueryClient();
  const [location, setLocation] = useState<Location>(loadLocation);
  const [helpOpen, setHelpOpen] = useState(false);
  const [howOpen, setHowOpen] = useState(false);

  // Show the "How it works" explainer once on first visit so a new operator
  // immediately understands the request pipeline and their part in it.
  useEffect(() => {
    if (typeof window === "undefined") return;
    if (!window.localStorage.getItem("aios.seen-howitworks")) {
      setHowOpen(true);
      window.localStorage.setItem("aios.seen-howitworks", "1");
    }
  }, []);

  // Clear cached data only when the signed-in user actually changes (account
  // switch), never on first mount. Running qc.clear() on mount races with the
  // child views' queries: effects fire child-first, so a view's useQuery
  // creates its query and starts fetching, then this parent effect wiped it,
  // leaving the view stuck loading forever. Track the previous id and skip the
  // initial run.
  const prevUserId = useRef(user?.id);
  useEffect(() => {
    if (prevUserId.current !== undefined && prevUserId.current !== user?.id) {
      qc.clear();
    }
    prevUserId.current = user?.id;
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

  const navigateToWorkflow = (requestId: string) => {
    setLocation({ section: "workflows", requestId, nodeId: null });
  };

  const selectNode = (nodeId: string | null) => {
    setLocation((prev) => ({ ...prev, nodeId }));
  };

  const section = location.section;

  return (
    <div className="h-screen w-screen overflow-hidden flex bg-[var(--color-bg)] text-[var(--color-fg)] md:pb-0 pb-16">
      <ShellRail
        active={section}
        onSelect={setSection}
        onBrandClick={() => setLocation({ section: "home", requestId: null, nodeId: null })}
        onUserClick={() => { qc.clear(); logout(); }}
      />

      {section === "home" && activeOrg && (
        <HomeView
          orgId={activeOrg.id}
          onOpenWorkflow={navigateToWorkflow}
          onNavigate={setSection}
          onShowHowItWorks={() => setHowOpen(true)}
        />
      )}
      {section === "my-work" && activeOrg && (
        <MyWorkView
          orgId={activeOrg.id}
          role={activeOrg.role}
          onOpenWorkflow={navigateToWorkflow}
        />
      )}

      {section === "requests" && activeOrg && (
        <RequestsView orgId={activeOrg.id} onOpenWorkflow={navigateToWorkflow} />
      )}

      {section === "workflows" && (
        <WorkflowView
          requestId={location.requestId}
          selectedNodeId={location.nodeId}
          onSelectNode={selectNode}
          onBack={() => setLocation({ section: "requests", requestId: null, nodeId: null })}
        />
      )}

      {section === "machines" && activeOrg && <MachinesView orgId={activeOrg.id} />}
      {section === "agents" && activeOrg && <AgentsView orgId={activeOrg.id} />}
      {section === "reports" && <ReportsView />}
      {section === "policies" && activeOrg && <PoliciesView orgId={activeOrg.id} />}
      {section === "integrations" && <IntegrationsView />}
      {section === "teams" && <OrgView />}
      {section === "settings" && <SettingsView />}

      <HelpOverlay
        open={helpOpen}
        onClose={() => setHelpOpen(false)}
        onHowItWorks={() => {
          setHelpOpen(false);
          setHowOpen(true);
        }}
      />
      <HowItWorks open={howOpen} onClose={() => setHowOpen(false)} onNavigate={setSection} />
    </div>
  );
}
