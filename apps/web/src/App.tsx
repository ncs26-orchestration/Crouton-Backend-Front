import { useEffect, useState } from "react";
import { ReactFlowProvider } from "@xyflow/react";
import { useQueryClient } from "@tanstack/react-query";

import { HelpOverlay } from "./components/HelpOverlay";
import { OrgSettingsModal } from "./components/OrgSettingsModal";
import { ProjectTree } from "./components/ProjectTree";
import { ShellRail, type ShellSection } from "./components/ShellRail";
import { ToastProvider } from "./components/Toasts";
import { ThemeProvider } from "./lib/theme";
import { api } from "./lib/api";
import type { Chat } from "./lib/types";
import { ChatView } from "./views/ChatView";
import { ProjectsHomeView } from "./views/ProjectsHomeView";
import { SettingsView } from "./views/SettingsView";
import { LoginView } from "./views/LoginView";
import { RegisterView } from "./views/RegisterView";
import { InboxView } from "./views/InboxView";
import { OrgView } from "./views/OrgView";
import { OrgSetupView } from "./views/OrgSetupView";
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
  projectId: string | null;
  chatId: string | null;
}

function loadLocation(): Location {
  if (typeof window === "undefined") {
    return { section: "home", projectId: null, chatId: null };
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return { section: "home", projectId: null, chatId: null };
    const parsed = JSON.parse(raw);
    const validSections: ShellSection[] = ["home", "inbox", "workflows", "agents", "settings", "help"];
    return {
      section: validSections.includes(parsed.section) ? parsed.section : "home",
      projectId: typeof parsed.projectId === "string" ? parsed.projectId : null,
      chatId: typeof parsed.chatId === "string" ? parsed.chatId : null,
    };
  } catch {
    return { section: "home", projectId: null, chatId: null };
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
  const [orgModalProject, setOrgModalProject] = useState<{ id: string; name: string } | null>(null);

  // Clear the query cache whenever the logged-in user changes so a
  // newly-registered (or switched) account never sees stale data from
  // the previous session.
  useEffect(() => {
    qc.clear();
  }, [user?.id, qc]);

  useEffect(() => {
    saveLocation(location);
  }, [location]);

  const setSection = (section: ShellSection) => {
    // Opening Help is transient — don't persist it; it pops a modal
    // and the previous section stays underneath.
    if (section === "help") {
      setHelpOpen(true);
      return;
    }
    setLocation((prev) => ({ ...prev, section }));
  };

  const openProject = (projectId: string) =>
    setLocation({ section: "home", projectId, chatId: null });

  const openChat = (chat: Chat) =>
    setLocation({ section: "home", projectId: chat.project_id, chatId: chat.id });

  const backToProjects = () =>
    setLocation({ section: "home", projectId: null, chatId: null });

  const openOrganisationSettings = () => {
    if (location.projectId) {
      setOrgModalProject({ id: location.projectId, name: "" });
    }
  };

  // "workflows" maps to home for now
  const effectiveSection = location.section === "workflows" ? "home" : location.section;

  return (
    <div className="h-screen w-screen overflow-hidden flex bg-[var(--color-bg)] text-[var(--color-fg)]">
      <ShellRail
        active={location.section}
        onSelect={setSection}
        onBrandClick={backToProjects}
        onUserClick={() => { qc.clear(); logout(); }}
      />

      {(effectiveSection === "home") && (
        <>
          <ProjectTree
            selectedProjectId={location.projectId}
            selectedChatId={location.chatId}
            onOpenProject={openProject}
            onOpenChat={openChat}
            onBackToProjects={backToProjects}
            onOrganisationSettings={location.projectId ? openOrganisationSettings : undefined}
          />
          {location.chatId ? (
            <ChatView chatId={location.chatId} />
          ) : (
            <ProjectsHomeView onOpenChat={openChat} onOpenProject={openProject} />
          )}
        </>
      )}

      {effectiveSection === "inbox" && <InboxView />}

      {effectiveSection === "agents" && <OrgView />}

      {effectiveSection === "settings" && (
        <SettingsView scopedProjectId={location.projectId} />
      )}

      <HelpOverlay open={helpOpen} onClose={() => setHelpOpen(false)} />

      {orgModalProject && (
        <OrgSettingsModalWrapper
          projectId={orgModalProject.id}
          onClose={() => setOrgModalProject(null)}
        />
      )}
    </div>
  );
}

function OrgSettingsModalWrapper({ projectId, onClose }: { projectId: string; onClose: () => void }) {
  const [orgModalProject, setOrgModalProject] = useState<{ id: string; name: string } | null>(null);

  useEffect(() => {
    api.getProject(projectId).then((data) => {
      setOrgModalProject({ id: projectId, name: data.project.name });
    });
  }, [projectId]);

  if (!orgModalProject) {
    return null;
  }

  return (
    <OrgSettingsModal
      projectId={orgModalProject.id}
      projectName={orgModalProject.name}
      overview={null}
      onClose={onClose}
      onSaved={onClose}
    />
  );
}
